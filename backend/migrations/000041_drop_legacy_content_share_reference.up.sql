-- Migration 000041: Drop legacy contents.share_reference column
--
-- Hard purge of the obsolete share_reference blob now that canonical
-- content_resource_occurrences + resource_projection are the only live
-- authority paths on content/feed/search surfaces.

ALTER TABLE contents
    DROP COLUMN IF EXISTS share_reference;
