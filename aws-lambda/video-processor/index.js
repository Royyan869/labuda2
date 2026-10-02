/**
 * AWS Lambda Function for Video Processing (koi major surface).
 *
 * Trigger: S3 ObjectCreated on videos/* (NOT images/*, NOT variants).
 * Runtime: Node.js 22.x, x86_64, Memory 1024 MB, Timeout 5 min.
 * Requires: ffmpeg Lambda layer providing /opt/bin/ffmpeg
 *   (attach in console; see README below). S3 read/write on labuda-uploads.
 *
 * Does, per mp4 upload:
 *  1. Remux to faststart (moov atom front) IN PLACE — S3 PUT is atomic, so
 *     readers see the old or the new object, never a partial file. No DB
 *     change is needed: keys stay identical.
 *  2. Extract the middle frame as {name}_poster.jpg next to the video —
 *     the key the backend VideoPosterKey rule derives. Mobile tiles render
 *     it; the player streams the remuxed mp4 progressively.
 *
 * Sizing contract: MAX_VIDEO_MB = 100 (== MediaUploadConfig.maxVideoSizeMb).
 * If the upload cap ever rises, raise Memory/Timeout here first.
 *
 * Skips: non-mp4, existing _poster.jpg, variant subfolders.
 */
const { S3Client, GetObjectCommand, PutObjectCommand } = require('@aws-sdk/client-s3');
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const execFileAsync = promisify(execFile);

const REGION = process.env.AWS_REGION || 'us-east-1';
const FFMPEG = process.env.FFMPEG_PATH || '/opt/bin/ffmpeg';
// Must stay == mobile MediaUploadConfig.maxVideoSizeMb.
const MAX_VIDEO_MB = 100;

const s3 = new S3Client({ region: REGION });

function isProcessableVideo(key) {
  const lower = key.toLowerCase();
  if (!lower.endsWith('.mp4')) return false;
  if (lower.endsWith('_poster.jpg')) return false;
  const segments = lower.split('/');
  if (segments.some((s) => ['thumbnail', 'medium', 'large', 'webp'].includes(s))) return false;
  return true;
}

function posterKey(key) {
  return key.replace(/\.mp4$/i, '') + '_poster.jpg';
}

async function ffprobeDuration(input) {
  const ffprobe = FFMPEG.replace(/ffmpeg$/, 'ffprobe');
  try {
    const { stdout } = await execFileAsync(ffprobe, [
      '-v', 'error', '-show_entries', 'format=duration',
      '-of', 'default=noprint_wrappers=1:nokey=1', input,
    ]);
    const seconds = parseFloat(stdout.trim());
    return Number.isFinite(seconds) && seconds > 0 ? seconds : 0;
  } catch {
    return 0;
  }
}

async function processRecord(record) {
  const bucket = record.s3.bucket.name;
  const key = decodeURIComponent(record.s3.object.key.replace(/\+/g, ' '));

  if (!isProcessableVideo(key)) {
    console.log(`Skipping: ${key}`);
    return { skipped: true, key };
  }

  const workdir = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'vid-'));
  const input = path.join(workdir, 'in.mp4');
  const remuxed = path.join(workdir, 'out.mp4');
  const poster = path.join(workdir, 'poster.jpg');

  try {
    const obj = await s3.send(new GetObjectCommand({ Bucket: bucket, Key: key }));
    const sizeMb = (obj.ContentLength || 0) / (1024 * 1024);
    if (sizeMb > MAX_VIDEO_MB * 1.5) {
      throw new Error(`video ${sizeMb.toFixed(1)}MB exceeds processing ceiling`);
    }
    const chunks = [];
    for await (const chunk of obj.Body) chunks.push(chunk);
    await fs.promises.writeFile(input, Buffer.concat(chunks));

    // 1. Faststart remux (no re-encode: quality untouched).
    await execFileAsync(FFMPEG, [
      '-y', '-i', input, '-c', 'copy', '-movflags', '+faststart', remuxed,
    ]);

    // 2. Middle-frame poster.
    const duration = await ffprobeDuration(input);
    const seek = duration > 0 ? (duration / 2).toFixed(2) : '1';
    await execFileAsync(FFMPEG, [
      '-y', '-ss', String(seek), '-i', input, '-vframes', '1',
      '-q:v', '3', poster,
    ]);

    const [remuxedBytes, posterBytes] = await Promise.all([
      fs.promises.readFile(remuxed),
      fs.promises.readFile(poster),
    ]);

    await s3.send(new PutObjectCommand({
      Bucket: bucket, Key: key, Body: remuxedBytes,
      ContentType: 'video/mp4', CacheControl: 'max-age=31536000',
    }));
    await s3.send(new PutObjectCommand({
      Bucket: bucket, Key: posterKey(key), Body: posterBytes,
      ContentType: 'image/jpeg', CacheControl: 'max-age=31536000',
    }));

    console.log(`Remuxed + poster done: ${key}`);
    return { success: true, key };
  } finally {
    await fs.promises.rm(workdir, { recursive: true, force: true });
  }
}

exports.handler = async (event) => {
  const results = await Promise.allSettled(
    (event.Records || []).map(processRecord),
  );
  const failed = results.filter((r) => r.status === 'rejected');
  failed.forEach((r) => console.error('Record failed:', r.reason));
  return {
    statusCode: failed.length > 0 ? 500 : 200,
    body: JSON.stringify({
      processed: results.filter((r) => r.status === 'fulfilled' && !r.value?.skipped).length,
      skipped: results.filter((r) => r.status === 'fulfilled' && r.value?.skipped).length,
      failed: failed.length,
    }),
  };
};
