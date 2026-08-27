/**
 * AWS Lambda Function for Basic Image Processing (No Sharp)
 *
 * Features:
 * - Copy original image to variant folders
 * - Generate basic metadata
 * - Setup structure for future Sharp upgrade
 * - Tag processed images
 */

import { S3Client, GetObjectCommand, PutObjectCommand, CopyObjectCommand, PutObjectTaggingCommand } from '@aws-sdk/client-s3';

const s3 = new S3Client({ region: 'ap-southeast-1' });

export const handler = async (event) => {
    console.log('Lambda triggered with event:', JSON.stringify(event, null, 2));

    try {
        // Process each S3 record
        for (const record of event.Records) {
            const bucket = record.s3.bucket.name;
            const key = decodeURIComponent(record.s3.object.key.replace(/\+/g, ' '));

            console.log(`Processing: ${bucket}/${key}`);

            // Skip if not an image or already a variant
            if (!isImageFile(key) || isVariantFile(key)) {
                console.log('Skipping:', key);
                continue;
            }

            console.log('✅ Image detected, processing:', key);

            // Get original image metadata
            const originalObject = await s3.send(new GetObjectCommand({
                Bucket: bucket,
                Key: key
            }));

            console.log('📊 Image size:', originalObject.ContentLength, 'bytes');
            console.log('📄 Content type:', originalObject.ContentType);

            // Create variants by copying original to different folders
            await createImageVariants(bucket, key, originalObject);

            // Tag original image as processed
            await tagAsProcessed(bucket, key);

            console.log('🎉 Successfully processed:', key);
        }

        return {
            statusCode: 200,
            body: JSON.stringify('Basic processing completed')
        };

    } catch (error) {
        console.error('❌ Error:', error);
        throw error;
    }
};

async function createImageVariants(bucket, originalKey, originalObject) {
    const variants = [
        { name: 'thumbnail', suffix: '_thumb' },
        { name: 'medium', suffix: '_med' },
        { name: 'large', suffix: '_large' }
    ];

    for (const variant of variants) {
        try {
            const variantKey = getVariantKey(originalKey, variant.name);

            // Copy original image to variant location
            // Note: Same size for now, will be actual resize when Sharp is added
            await s3.send(new CopyObjectCommand({
                Bucket: bucket,
                CopySource: `${bucket}/${originalKey}`,
                Key: variantKey,
                MetadataDirective: 'REPLACE',
                ContentType: originalObject.ContentType,
                CacheControl: 'max-age=31536000', // 1 year cache
                Metadata: {
                    'original-key': originalKey,
                    'variant': variant.name,
                    'processing': 'basic-copy',
                    'created-at': new Date().toISOString()
                }
            }));

            console.log(`📁 Created variant: ${variantKey}`);

        } catch (error) {
            console.error(`❌ Failed to create ${variant.name}:`, error.message);
            // Continue with other variants
        }
    }
}

async function tagAsProcessed(bucket, key) {
    try {
        await s3.send(new PutObjectTaggingCommand({
            Bucket: bucket,
            Key: key,
            Tagging: {
                TagSet: [
                    { Key: 'processed', Value: 'true' },
                    { Key: 'processor', Value: 'labuda-lambda-basic' },
                    { Key: 'processedAt', Value: new Date().toISOString() },
                    { Key: 'variants', Value: 'thumbnail,medium,large' }
                ]
            }
        }));
        console.log('🏷️ Tagged as processed:', key);
    } catch (error) {
        console.error('⚠️ Failed to tag (non-critical):', error.message);
        // Non-critical error - don't throw
    }
}

function isImageFile(key) {
    const imageExtensions = ['.jpg', '.jpeg', '.png', '.gif', '.webp'];
    const extension = key.toLowerCase().substr(key.lastIndexOf('.'));
    return imageExtensions.includes(extension);
}

function isVariantFile(key) {
    return key.includes('/thumbnail/') ||
           key.includes('/medium/') ||
           key.includes('/large/') ||
           key.includes('/webp/');
}

function getVariantKey(originalKey, variant) {
    // Transform: images/photo.jpg → images/thumbnail/photo.jpg
    const parts = originalKey.split('/');
    const filename = parts.pop();
    const path = parts.join('/');
    return `${path}/${variant}/${filename}`;
}

// Helper function to estimate what Sharp would do (for logging)
function getTargetDimensions(variant) {
    const sizes = {
        thumbnail: { width: 150, height: 150 },
        medium: { width: 600, height: 600 },
        large: { width: 1200, height: 1200 }
    };
    return sizes[variant] || sizes.medium;
}