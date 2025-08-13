import {API_BASE_URL} from './apiService';

export interface UploadedFile {
    id: string;
    name: string;
    path: string;
    size: number;
    mimeType: string;
}

export interface BatchUploadResult {
    uploads: UploadedFile[];
    count: number;
    path: string;
}

export class UploadService {
    /**
     * Upload a single file
     */
    static async uploadFile(file: File): Promise<UploadedFile> {
        const formData = new FormData();
        formData.append('file', file);

        const response = await fetch(`${API_BASE_URL}/api/upload`, {
            method: 'POST',
            body: formData,
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(`Upload failed: ${error}`);
        }

        return response.json();
    }

    /**
     * Upload multiple files at once
     */
    static async uploadBatch(files: File[]): Promise<BatchUploadResult> {
        const formData = new FormData();

        files.forEach((file, index) => {
            formData.append(`file${index}`, file);
        });

        const response = await fetch(`${API_BASE_URL}/api/upload/batch`, {
            method: 'POST',
            body: formData,
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(`Batch upload failed: ${error}`);
        }

        return response.json();
    }

    /**
     * Upload a directory (multiple files maintaining structure)
     */
    static async uploadDirectory(files: FileList): Promise<BatchUploadResult> {
        const formData = new FormData();

        // Process files with their paths
        Array.from(files).forEach((file) => {
            // @ts-ignore - webkitRelativePath is available in browsers
            const path = file.webkitRelativePath || file.name;
            formData.append(path, file);
        });

        const response = await fetch(`${API_BASE_URL}/api/upload/batch`, {
            method: 'POST',
            body: formData,
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(`Directory upload failed: ${error}`);
        }

        return response.json();
    }

    /**
     * Clean up uploaded files
     */
    static async cleanup(uploadId: string): Promise<void> {
        const response = await fetch(`${API_BASE_URL}/api/upload/cleanup?id=${uploadId}`, {
            method: 'DELETE',
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(`Cleanup failed: ${error}`);
        }
    }

    /**
     * Convert File objects to upload paths for job creation
     */
    static async prepareFilesForJob(files: File[]): Promise<string[]> {
        if (files.length === 0) return [];

        // Upload files and get their paths
        const uploadResult = await this.uploadBatch(files);
        return uploadResult.uploads.map(upload => upload.path);
    }

    /**
     * Check if a file type is allowed for upload
     */
    static isAllowedFileType(file: File): boolean {
        // Define allowed file extensions
        const allowedExtensions = [
            '.py', '.js', '.ts', '.sh', '.yaml', '.yml', '.json', '.txt',
            '.csv', '.parquet', '.h5', '.hdf5', '.pkl', '.pickle',
            '.tar', '.gz', '.zip', '.bz2',
            '.png', '.jpg', '.jpeg', '.gif', '.svg',
            '.model', '.weights', '.onnx', '.pb', '.pth', '.pt'
        ];

        const fileName = file.name.toLowerCase();
        return allowedExtensions.some(ext => fileName.endsWith(ext));
    }

    /**
     * Format file size for display
     */
    static formatFileSize(bytes: number): string {
        if (bytes === 0) return '0 Bytes';

        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));

        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    /**
     * Validate file size (max 100MB)
     */
    static validateFileSize(file: File): boolean {
        const maxSize = 100 * 1024 * 1024; // 100MB
        return file.size <= maxSize;
    }
}