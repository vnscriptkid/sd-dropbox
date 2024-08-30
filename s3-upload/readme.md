# Main flows
- 1. Initialize upload API: `initUpload(namespace, relativePath, fileName, chunkHashes[]): chunkHashes[]`
    - Check if the file already exists in `upload_metadata` based on (namespace, relativePath, fileName)
        - If exists, create entry with version += 1
            - Loop through chunkHashes[] of old version
                - If chunk exists in `upload_chunks`, clone chunk but point to the new `upload_metadata` entry (s3 url remains the same)
                - If chunk does not exist in `upload_chunks`, create new entry
            - Return chunkHashes[] of ones that were not found in the old version, only those need to be uploaded
        - If not exists, create entry with version = 1
            - Loop through chunkHashes[] and create entry in `upload_chunks` for each chunk
            - Return chunkHashes[]

- 2. Init chunk upload API: `initChunkUpload(uploadId, chunkHash): signedUrl`
    - Find the `upload_metadata` entry based on uploadId
    - Find the `upload_chunks` entry based on chunkHash
    - Call S3 API to get signed URL for the chunk
    - Update the `upload_chunks` entry with the signed URL
    - Return the signed URL

- 3. Complete chunk upload API: `completeChunkUpload(uploadId, chunkHash): 200`
    - Find the `upload_metadata` entry based on uploadId
    - Find the `upload_chunks` entry based on chunkHash
    - Update the `upload_chunks` entry with status = 'completed'
    - Check if all chunks are uploaded
        - If yes, update the `upload_metadata` entry with status = 'completed'
        - If no, return


- 4. Sync upload API: `syncUpload(namespace, existingUploadMetadata[]): UploadMetadata[]`
    - Find all latest files in `upload_metadata` group by (namespace, relativePath, fileName)
```sql
-- use `distinct on` in postgres and get latest entry based on version
SELECT DISTINCT ON (namespace, relative_path, file_name) *
FROM upload_metadata
WHERE namespace = 'vnscriptkid'
ORDER BY namespace, relative_path, file_name, version DESC;


-- seed data
INSERT INTO upload_metadata (namespace, relative_path, file_name, version, status, upload_id)
VALUES ('vnscriptkid', '/parent', 'index.html', 1, 'completed', '5f58c81f-d9e2-48b9-8119-d65117e3de1a'),
        ('vnscriptkid', '/parent', 'names.txt', 1, 'completed', '5f58c81f-d9e2-48b9-8119-d65117e3de1b'), -- << latest
        ('vnscriptkid', '/parent/child', 'report.json', 1, 'completed', '5f58c81f-d9e2-48b9-8119-d65117e3de1c'),
        ('vnscriptkid', '/parent', 'index.html', 2, 'completed', '5f58c81f-d9e2-48b9-8119-d65117e3de1d'),
        ('vnscriptkid', '/parent/child', 'report.json', 2, 'completed', '5f58c81f-d9e2-48b9-8119-d65117e3de1e'), -- << latest
        ('vnscriptkid', '/parent', 'index.html', 3, 'completed', '5f58c81f-d9e2-48b9-8119-d65117e3de1f'), -- << latest
        ('randomuser', '/internal', 'cat.png', 1, 'completed', '5f58c81f-d9e2-48b9-8119-d65117e3de2a');
```
    - Loop through latest entries
        - If entry is not in existingUploadMetadata[], add to return list (new upload)
        - If entry is in existingUploadMetadata[], check if version is different
            - If yes, add to return list
            - If no, skip it
    - Return the list of new uploads
