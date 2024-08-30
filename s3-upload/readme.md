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
