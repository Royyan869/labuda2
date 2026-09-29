/**
 * AWS Lambda Function for Image Processing
 *
 * Features:
 * - Auto-generate multiple image sizes (thumbnail, medium, large)
 * - Convert to WebP format
 *
 * Non-goals (dead outputs, purged): blurhash generation, S3 tagging,
 * DynamoDB metadata — no consumer reads them.
 *
 * Trigger: S3 PUT events
 * Runtime: Node.js 18.x or higher
 */

const AWS = require('aws-sdk');
const sharp = require('sharp');

const s3 = new AWS.S3();

// Configuration
const IMAGE_SIZES = {
  thumbnail: { width: 150, height: 150, quality: 80 },
  medium: { width: 600, height: 600, quality: 85 },
  large: { width: 1200, height: 1200, quality: 90 }
};

const SUPPORTED_FORMATS = ['.jpg', '.jpeg', '.png', '.gif', '.webp'];

exports.handler = async (event) => {
  console.log('Lambda triggered:', JSON.stringify(event, null, 2));

  try {
    // Process each S3 event record
    const promises = event.Records.map(record => processImage(record));
    const results = await Promise.allSettled(promises);

    // Log results
    results.forEach((result, index) => {
      if (result.status === 'rejected') {
        console.error(`Failed to process record ${index}:`, result.reason);
      }
    });

    return {
      statusCode: 200,
      body: JSON.stringify({
        message: 'Image processing completed',
        processed: results.filter(r => r.status === 'fulfilled').length,
        failed: results.filter(r => r.status === 'rejected').length
      })
    };
  } catch (error) {
    console.error('Lambda error:', error);
    throw error;
  }
};

async function processImage(record) {
  const bucket = record.s3.bucket.name;
  const key = decodeURIComponent(record.s3.object.key.replace(/\+/g, ' '));

  console.log(`Processing image: ${bucket}/${key}`);

  // Skip if not an image, already a variant, or a fixed identity photo
  if (!isImageFile(key) || isVariantFile(key) || isFixedIdentityFile(key)) {
    console.log(`Skipping: ${key}`);
    return;
  }

  try {
    // Get original image from S3
    const originalImage = await s3.getObject({
      Bucket: bucket,
      Key: key
    }).promise();

    // Load image with sharp
    const image = sharp(originalImage.Body);
    const metadata = await image.metadata();

    console.log(`Image metadata:`, metadata);

    // Generate all variants in parallel.
    // NOTE: blurhash + S3 tagging were removed — no consumer reads them
    // (mobile generates blurhash client-side; backend has no blurhash
    // column), and the tag write failed on blurhash characters.
    const processingTasks = [
      ...generateSizeVariants(bucket, key, originalImage.Body, metadata),
      generateWebPVersion(bucket, key, originalImage.Body, metadata),
    ];

    await Promise.allSettled(processingTasks);

    console.log(`Successfully processed: ${key}`);
    return { success: true, key };

  } catch (error) {
    console.error(`Error processing ${key}:`, error);
    throw error;
  }
}

function generateSizeVariants(bucket, key, imageBuffer, metadata) {
  return Object.entries(IMAGE_SIZES).map(([size, config]) =>
    generateVariant(bucket, key, imageBuffer, size, config, metadata)
  );
}

async function generateVariant(bucket, key, imageBuffer, sizeName, config, metadata) {
  try {
    const variantKey = getVariantKey(key, sizeName);

    // Skip if variant is larger than original
    if (config.width > metadata.width && config.height > metadata.height) {
      console.log(`Skipping ${sizeName} - original is smaller`);
      return;
    }

    // Process image
    const processedImage = await sharp(imageBuffer)
      .resize(config.width, config.height, {
        fit: 'inside',
        withoutEnlargement: true
      })
      .jpeg({ quality: config.quality, progressive: true })
      .toBuffer();

    // Upload to S3
    await s3.putObject({
      Bucket: bucket,
      Key: variantKey,
      Body: processedImage,
      ContentType: 'image/jpeg',
      CacheControl: 'max-age=31536000', // 1 year cache
      Metadata: {
        'original-key': key,
        'variant': sizeName
      }
    }).promise();

    console.log(`Created variant: ${variantKey}`);
    return { variant: sizeName, key: variantKey };

  } catch (error) {
    console.error(`Failed to create ${sizeName} variant:`, error);
    throw error;
  }
}

async function generateWebPVersion(bucket, key, imageBuffer, metadata) {
  try {
    const webpKey = getWebPKey(key);

    const webpImage = await sharp(imageBuffer)
      .webp({ quality: 85 })
      .toBuffer();

    await s3.putObject({
      Bucket: bucket,
      Key: webpKey,
      Body: webpImage,
      ContentType: 'image/webp',
      CacheControl: 'max-age=31536000',
      Metadata: {
        'original-key': key,
        'variant': 'webp'
      }
    }).promise();

    console.log(`Created WebP version: ${webpKey}`);
    return { variant: 'webp', key: webpKey };

  } catch (error) {
    console.error('Failed to create WebP version:', error);
    throw error;
  }
}

// Helper functions
function isImageFile(key) {
  const ext = key.toLowerCase().substr(key.lastIndexOf('.'));
  return SUPPORTED_FORMATS.includes(ext);
}

function isVariantFile(key) {
  return key.includes('/thumbnail/') ||
         key.includes('/medium/') ||
         key.includes('/large/') ||
         key.includes('/webp/');
}

// Fixed-key identity photos (avatars, stores, covers) never need variants:
// small, fixed-size, overwritten in place. Skipping them here keeps the
// pipeline scoped to domain content (content/commerce/chat/evidence).
function isFixedIdentityFile(key) {
  return key.startsWith('images/avatars/') ||
         key.startsWith('images/stores/') ||
         key.startsWith('images/profile-covers/');
}

function getVariantKey(originalKey, variant) {
  const parts = originalKey.split('/');
  const filename = parts.pop();
  const path = parts.join('/');
  return `${path}/${variant}/${filename}`;
}

function getWebPKey(originalKey) {
  const parts = originalKey.split('/');
  const filename = parts.pop();
  const nameWithoutExt = filename.substr(0, filename.lastIndexOf('.'));
  const path = parts.join('/');
  return `${path}/webp/${nameWithoutExt}.webp`;
}

