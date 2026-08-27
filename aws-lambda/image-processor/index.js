/**
 * AWS Lambda Function for Image Processing
 *
 * Features:
 * - Auto-generate multiple image sizes (thumbnail, medium, large)
 * - Convert to WebP format
 * - Generate blurhash
 * - Update metadata in DynamoDB (optional)
 *
 * Trigger: S3 PUT events
 * Runtime: Node.js 18.x or higher
 */

const AWS = require('aws-sdk');
const sharp = require('sharp');
const { encode } = require('blurhash');

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

  // Skip if not an image or already a variant
  if (!isImageFile(key) || isVariantFile(key)) {
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

    // Generate all variants in parallel
    const processingTasks = [
      ...generateSizeVariants(bucket, key, originalImage.Body, metadata),
      generateWebPVersion(bucket, key, originalImage.Body, metadata),
      generateBlurhash(originalImage.Body, metadata)
    ];

    const results = await Promise.allSettled(processingTasks);

    // Extract blurhash from results
    const blurhashResult = results.find(r =>
      r.status === 'fulfilled' && r.value?.blurhash
    );

    // Store metadata (optional - if using DynamoDB)
    if (blurhashResult?.value?.blurhash) {
      await storeImageMetadata(bucket, key, {
        blurhash: blurhashResult.value.blurhash,
        dimensions: {
          width: metadata.width,
          height: metadata.height
        },
        format: metadata.format,
        variants: getVariantKeys(key)
      });
    }

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

async function generateBlurhash(imageBuffer, metadata) {
  try {
    // Resize image for blurhash (max 64x64 for performance)
    const resized = await sharp(imageBuffer)
      .resize(64, 64, { fit: 'inside' })
      .raw()
      .ensureAlpha()
      .toBuffer();

    // Calculate dimensions after resize
    const aspectRatio = metadata.width / metadata.height;
    let blurWidth = 64;
    let blurHeight = 64;

    if (aspectRatio > 1) {
      blurHeight = Math.round(64 / aspectRatio);
    } else {
      blurWidth = Math.round(64 * aspectRatio);
    }

    // Generate blurhash
    const blurhash = encode(
      new Uint8ClampedArray(resized),
      blurWidth,
      blurHeight,
      4, // componentX
      3  // componentY
    );

    console.log(`Generated blurhash: ${blurhash}`);
    return { blurhash };

  } catch (error) {
    console.error('Failed to generate blurhash:', error);
    throw error;
  }
}

async function storeImageMetadata(bucket, key, metadata) {
  // Option 1: Store in S3 object tags
  try {
    await s3.putObjectTagging({
      Bucket: bucket,
      Key: key,
      Tagging: {
        TagSet: [
          { Key: 'blurhash', Value: metadata.blurhash.substring(0, 128) }, // Tags have length limit
          { Key: 'processed', Value: 'true' }
        ]
      }
    }).promise();

    console.log(`Stored metadata for: ${key}`);
  } catch (error) {
    console.error('Failed to store metadata:', error);
    // Non-critical error - don't throw
  }

  // Option 2: Store in DynamoDB (recommended for production)
  // const dynamodb = new AWS.DynamoDB.DocumentClient();
  // await dynamodb.put({
  //   TableName: 'ImageMetadata',
  //   Item: {
  //     key,
  //     bucket,
  //     ...metadata,
  //     processedAt: new Date().toISOString()
  //   }
  // }).promise();
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

function getVariantKeys(originalKey) {
  return {
    thumbnail: getVariantKey(originalKey, 'thumbnail'),
    medium: getVariantKey(originalKey, 'medium'),
    large: getVariantKey(originalKey, 'large'),
    webp: getWebPKey(originalKey)
  };
}