import React, {useCallback, useEffect, useRef, useState} from 'react';
import {JobConfig, JobExecuteRequest} from '../../types/job';
import {CommandBuilder} from '../../services/commandBuilder';
import {useJobs} from '../../hooks/useJobs';
import {File, FolderPlus, Play, RotateCcw, Save, Trash2, Upload} from 'lucide-react';
import clsx from 'clsx';
import {UploadedFile, UploadService} from '../../services/uploadService';

interface SimpleJobBuilderProps {
    onJobCreated?: (jobId: string) => void;
    onClose?: () => void;
}

export const SimpleJobBuilder: React.FC<SimpleJobBuilderProps> = ({
                                                                      onJobCreated,
                                                                      onClose
                                                                  }) => {
    const {executeJob, loading} = useJobs();
    const [config, setConfig] = useState<JobConfig>({
        command: '',
        files: [],
        directories: [],
        maxCpu: 0,
        maxMemory: 0,
        cpuCores: '',
        maxIobps: 0,
        runtime: '',
        network: 'bridge',
        volumes: [],
        envVars: {},
        secretEnvVars: {},
        schedule: ''
    });

    const [preview, setPreview] = useState<string>('');
    const [showAdvanced, setShowAdvanced] = useState(false);
    const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([]);
    const [uploadError, setUploadError] = useState<string>('');
    const [isDragOver, setIsDragOver] = useState<boolean>(false);
    const fileInputRef = useRef<HTMLInputElement>(null);
    const dirInputRef = useRef<HTMLInputElement>(null);
    const dropZoneRef = useRef<HTMLDivElement>(null);

    const updateConfig = useCallback((updates: Partial<JobConfig>) => {
        setConfig(prev => ({...prev, ...updates}));
    }, []);

    const updateEnvVar = useCallback((key: string, value: string) => {
        setConfig(prev => ({
            ...prev,
            envVars: {...prev.envVars, [key]: value}
        }));
    }, []);

    const removeEnvVar = useCallback((key: string) => {
        setConfig(prev => {
            const newEnvVars = {...prev.envVars};
            delete newEnvVars[key];
            return {...prev, envVars: newEnvVars};
        });
    }, []);

    const updateSecretEnvVar = useCallback((key: string, value: string) => {
        setConfig(prev => ({
            ...prev,
            secretEnvVars: {...prev.secretEnvVars, [key]: value}
        }));
    }, []);

    const removeSecretEnvVar = useCallback((key: string) => {
        setConfig(prev => {
            const newSecretEnvVars = {...prev.secretEnvVars};
            delete newSecretEnvVars[key];
            return {...prev, secretEnvVars: newSecretEnvVars};
        });
    }, []);

    const addVolume = useCallback((volume: string) => {
        if (volume && !config.volumes.includes(volume)) {
            setConfig(prev => ({
                ...prev,
                volumes: [...prev.volumes, volume]
            }));
        }
    }, [config.volumes]);

    const removeVolume = useCallback((volume: string) => {
        setConfig(prev => ({
            ...prev,
            volumes: prev.volumes.filter(v => v !== volume)
        }));
    }, []);

    const handleFileUpload = useCallback(async (files: FileList | null) => {
        if (!files || files.length === 0) return;

        setUploadError('');
        const validFiles: File[] = [];

        // Validate files
        for (let i = 0; i < files.length; i++) {
            const file = files[i];
            if (!UploadService.validateFileSize(file)) {
                setUploadError(`File ${file.name} exceeds 100MB limit`);
                continue;
            }
            if (!UploadService.isAllowedFileType(file)) {
                setUploadError(`File type not allowed: ${file.name}`);
                continue;
            }
            validFiles.push(file);
        }

        if (validFiles.length === 0) return;

        try {
            // Upload files
            const result = await UploadService.uploadBatch(validFiles);
            setUploadedFiles(prev => [...prev, ...result.uploads]);

            // Update config with file paths
            const filePaths = result.uploads.map(f => f.path);
            setConfig(prev => ({
                ...prev,
                files: [...prev.files, ...filePaths]
            }));
        } catch (error) {
            setUploadError(`Upload failed: ${error}`);
        }
    }, []);

    const handleDirectoryUpload = useCallback(async (files: FileList | null) => {
        if (!files || files.length === 0) return;

        setUploadError('');

        try {
            const result = await UploadService.uploadDirectory(files);
            setUploadedFiles(prev => [...prev, ...result.uploads]);

            // Update config with directory path
            setConfig(prev => ({
                ...prev,
                directories: [...prev.directories, result.path]
            }));
        } catch (error) {
            setUploadError(`Directory upload failed: ${error}`);
        }
    }, []);

    const removeUploadedFile = useCallback((fileId: string) => {
        const file = uploadedFiles.find(f => f.id === fileId);
        if (!file) return;

        // Remove from uploaded files
        setUploadedFiles(prev => prev.filter(f => f.id !== fileId));

        // Remove from config
        setConfig(prev => ({
            ...prev,
            files: prev.files.filter(f => f !== file.path)
        }));

        // Clean up on server
        UploadService.cleanup(fileId).catch(console.error);
    }, [uploadedFiles]);

    // Drag and drop handlers
    const handleDragEnter = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragOver(true);
    }, []);

    const handleDragLeave = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        // Only set dragOver to false if we're leaving the drop zone entirely
        if (dropZoneRef.current && !dropZoneRef.current.contains(e.relatedTarget as Node)) {
            setIsDragOver(false);
        }
    }, []);

    const handleDragOver = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
    }, []);

    const handleDrop = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragOver(false);

        const files = e.dataTransfer.files;
        if (files && files.length > 0) {
            // Check if this looks like a directory structure (files have path separators)
            const hasDirectoryStructure = Array.from(files).some(file => {
                const fileWithPath = file as File & { webkitRelativePath?: string };
                return fileWithPath.webkitRelativePath && fileWithPath.webkitRelativePath.includes('/');
            });

            if (hasDirectoryStructure) {
                handleDirectoryUpload(files);
            } else {
                handleFileUpload(files);
            }
        }
    }, [handleFileUpload, handleDirectoryUpload]);

    const updatePreview = useCallback(() => {
        try {
            const generated = CommandBuilder.fromJobConfig(config);
            setPreview(generated.fullCommand);
        } catch (error) {
            setPreview('# Error generating command preview');
        }
    }, [config]);

    useEffect(() => {
        updatePreview();
    }, [updatePreview]);

    const handleSubmit = useCallback(async (e: React.FormEvent) => {
        e.preventDefault();

        if (!config.command.trim()) {
            alert('Command is required');
            return;
        }

        try {
            const request: JobExecuteRequest = {
                command: config.command,
                maxCPU: config.maxCpu || undefined,
                maxMemory: config.maxMemory || undefined,
                maxIOBPS: config.maxIobps || undefined,
                cpuCores: config.cpuCores || undefined,
                runtime: config.runtime || undefined,
                network: config.network,
                volumes: config.volumes,
                uploads: config.files, // These are now actual file paths from upload handler
                uploadDirs: config.directories, // These are now actual directory paths
                envVars: config.envVars,
                secretEnvVars: config.secretEnvVars,
                schedule: config.schedule || undefined
            };

            const jobId = await executeJob(request);
            onJobCreated?.(jobId);

            // Reset form
            setConfig({
                command: '',
                files: [],
                directories: [],
                maxCpu: 0,
                maxMemory: 0,
                cpuCores: '',
                maxIobps: 0,
                runtime: '',
                network: 'bridge',
                volumes: [],
                envVars: {},
                secretEnvVars: {},
                schedule: ''
            });
            setUploadedFiles([]);
            setUploadError('');
        } catch (error) {
            alert(`Failed to create job: ${error instanceof Error ? error.message : 'Unknown error'}`);
        }
    }, [config, executeJob, onJobCreated]);

    const resetForm = useCallback(() => {
        setConfig({
            command: '',
            files: [],
            directories: [],
            maxCpu: 0,
            maxMemory: 0,
            cpuCores: '',
            maxIobps: 0,
            runtime: '',
            network: 'bridge',
            volumes: [],
            envVars: {},
            secretEnvVars: {},
            schedule: ''
        });
    }, []);

    return (
        <div className="max-w-4xl mx-auto p-6">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg">
                <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div className="flex items-center justify-between">
                        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Create New Job</h2>
                        {onClose && (
                            <button
                                onClick={onClose}
                                className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300"
                            >
                                ✕
                            </button>
                        )}
                    </div>
                </div>

                <form onSubmit={handleSubmit} className="p-6">
                    <div className="space-y-6">
                        {/* Basic Configuration */}
                        <div>
                            <label className="block text-sm font-medium text-white mb-2">
                                Command *
                            </label>
                            <input
                                type="text"
                                value={config.command}
                                onChange={(e) => updateConfig({command: e.target.value})}
                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                placeholder="python3 script.py --epochs=100"
                                required
                            />
                            <p className="mt-1 text-xs text-gray-500">
                                The command to execute in the job container
                            </p>
                        </div>


                        {/* File Uploads */}
                        <div>
                            <label className="block text-sm font-medium text-white mb-2">
                                Files & Directories
                            </label>
                            <div
                                ref={dropZoneRef}
                                onDragEnter={handleDragEnter}
                                onDragLeave={handleDragLeave}
                                onDragOver={handleDragOver}
                                onDrop={handleDrop}
                                className={clsx(
                                    "border-2 border-dashed rounded-lg p-6 text-center transition-all duration-200",
                                    isDragOver
                                        ? "border-blue-400 bg-blue-50 scale-[1.02]"
                                        : "border-gray-300 hover:border-gray-400"
                                )}
                            >
                                <Upload className={clsx(
                                    "w-8 h-8 mx-auto mb-2 transition-colors",
                                    isDragOver ? "text-blue-500" : "text-gray-400"
                                )}/>
                                <p className={clsx(
                                    "text-sm mb-2 transition-colors",
                                    isDragOver ? "text-blue-700 font-medium" : "text-gray-600"
                                )}>
                                    {isDragOver
                                        ? "Release to upload files..."
                                        : "Drop files or directories here, or click to browse"
                                    }
                                </p>
                                <div className="space-x-2">
                                    <input
                                        ref={fileInputRef}
                                        type="file"
                                        multiple
                                        onChange={(e) => handleFileUpload(e.target.files)}
                                        className="hidden"
                                        accept=".py,.js,.ts,.sh,.yaml,.yml,.json,.txt,.csv,.parquet,.h5,.tar,.gz,.zip"
                                    />
                                    <input
                                        ref={dirInputRef}
                                        type="file"
                                        {...{
                                            webkitdirectory: "true",
                                            directory: "true"
                                        } as React.InputHTMLAttributes<HTMLInputElement>}
                                        multiple
                                        onChange={(e) => handleDirectoryUpload(e.target.files)}
                                        className="hidden"
                                    />
                                    <button
                                        type="button"
                                        onClick={() => fileInputRef.current?.click()}
                                        className="inline-flex items-center px-3 py-1 border border-gray-300 rounded text-sm hover:bg-gray-50"
                                    >
                                        <Upload className="w-4 h-4 mr-1"/>
                                        Add Files
                                    </button>
                                    <button
                                        type="button"
                                        onClick={() => dirInputRef.current?.click()}
                                        className="inline-flex items-center px-3 py-1 border border-gray-300 rounded text-sm hover:bg-gray-50"
                                    >
                                        <FolderPlus className="w-4 h-4 mr-1"/>
                                        Add Directory
                                    </button>
                                </div>

                                {/* Upload Error */}
                                {uploadError && (
                                    <div className="mt-3 text-sm text-red-600 bg-red-50 p-2 rounded">
                                        {uploadError}
                                    </div>
                                )}

                                {/* Uploaded Files List */}
                                {uploadedFiles.length > 0 && (
                                    <div className="mt-4 text-left space-y-2">
                                        <p className="text-xs text-gray-500 font-medium">Uploaded Files:</p>
                                        {uploadedFiles.map((file) => (
                                            <div key={file.id}
                                                 className="flex items-center justify-between text-sm bg-gray-50 p-2 rounded">
                                                <div className="flex items-center space-x-2">
                                                    <File className="w-4 h-4 text-gray-400"/>
                                                    <span className="text-gray-700">{file.name}</span>
                                                    <span className="text-xs text-gray-500">
                            ({UploadService.formatFileSize(file.size)})
                          </span>
                                                </div>
                                                <button
                                                    type="button"
                                                    onClick={() => removeUploadedFile(file.id)}
                                                    className="text-red-500 hover:text-red-700"
                                                >
                                                    <Trash2 className="w-4 h-4"/>
                                                </button>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                        </div>

                        {/* Resource Limits */}
                        <div>
                            <h3 className="text-lg font-medium text-gray-500 mb-4">Resource Limits</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-sm font-medium text-white mb-2">
                                        CPU Limit (%)
                                    </label>
                                    <input
                                        type="number"
                                        value={config.maxCpu || ''}
                                        onChange={(e) => updateConfig({maxCpu: parseInt(e.target.value) || 0})}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        placeholder="200"
                                        min="0"
                                    />
                                    <p className="mt-1 text-xs text-gray-500">
                                        CPU limit as percentage (200% = 2 cores)
                                    </p>
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-white mb-2">
                                        Memory Limit (MB)
                                    </label>
                                    <input
                                        type="number"
                                        value={config.maxMemory || ''}
                                        onChange={(e) => updateConfig({maxMemory: parseInt(e.target.value) || 0})}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        placeholder="2048"
                                        min="0"
                                    />
                                    <p className="mt-1 text-xs text-gray-500">
                                        Memory limit in megabytes
                                    </p>
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-white mb-2">
                                        CPU Cores
                                    </label>
                                    <input
                                        type="text"
                                        value={config.cpuCores}
                                        onChange={(e) => updateConfig({cpuCores: e.target.value})}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        placeholder="0-3"
                                    />
                                    <p className="mt-1 text-xs text-gray-500">
                                        CPU cores to use (e.g., "0-3" or "0,2,4")
                                    </p>
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-white mb-2">
                                        I/O Limit (bytes/sec)
                                    </label>
                                    <input
                                        type="number"
                                        value={config.maxIobps || ''}
                                        onChange={(e) => updateConfig({maxIobps: parseInt(e.target.value) || 0})}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        placeholder="10485760"
                                        min="0"
                                    />
                                    <p className="mt-1 text-xs text-gray-500">
                                        I/O bandwidth limit in bytes per second
                                    </p>
                                </div>
                            </div>
                        </div>

                        {/* Environment */}
                        <div>
                            <h3 className="text-lg font-medium text-gray-500 mb-4">Environment</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-sm font-medium text-white mb-2">
                                        Runtime
                                    </label>
                                    <select
                                        value={config.runtime}
                                        onChange={(e) => updateConfig({runtime: e.target.value})}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    >
                                        <option value="">Default</option>
                                        <option value="python:3.11-ml">Python 3.11 ML</option>
                                    </select>
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-white mb-2">
                                        Network
                                    </label>
                                    <select
                                        value={config.network}
                                        onChange={(e) => updateConfig({network: e.target.value})}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    >
                                        <option value="bridge">Bridge (default)</option>
                                        <option value="host">Host</option>
                                        <option value="none">None</option>
                                        <option value="isolated">Isolated</option>
                                    </select>
                                </div>
                            </div>
                        </div>

                        {/* Volumes */}
                        <div>
                            <label className="block text-sm font-medium text-white mb-2">
                                Volumes
                            </label>
                            <div className="flex space-x-2 mb-2">
                                <input
                                    type="text"
                                    placeholder="Volume name"
                                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    onKeyPress={(e) => {
                                        if (e.key === 'Enter') {
                                            addVolume((e.target as HTMLInputElement).value);
                                            (e.target as HTMLInputElement).value = '';
                                        }
                                    }}
                                />
                                <button
                                    type="button"
                                    onClick={(e) => {
                                        const input = e.currentTarget.previousElementSibling as HTMLInputElement;
                                        addVolume(input.value);
                                        input.value = '';
                                    }}
                                    className="px-3 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
                                >
                                    Add
                                </button>
                            </div>
                            {config.volumes.length > 0 && (
                                <div className="space-y-1">
                                    {config.volumes.map((volume, index) => (
                                        <div key={index}
                                             className="flex items-center justify-between bg-gray-50 px-3 py-2 rounded">
                                            <span className="text-sm">{volume}</span>
                                            <button
                                                type="button"
                                                onClick={() => removeVolume(volume)}
                                                className="text-red-500 hover:text-red-700"
                                            >
                                                Remove
                                            </button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>

                        {/* Advanced Options */}
                        <div>
                            <button
                                type="button"
                                onClick={() => setShowAdvanced(!showAdvanced)}
                                className="text-blue-600 hover:text-blue-800 text-sm font-medium"
                            >
                                {showAdvanced ? 'Hide' : 'Show'} Advanced Options
                            </button>

                            {showAdvanced && (
                                <div className="mt-4 space-y-4 pt-4 border-t border-gray-200">

                                    <div>
                                        <label className="block text-sm font-medium text-white mb-2">
                                            Schedule (cron format)
                                        </label>
                                        <input
                                            type="text"
                                            value={config.schedule}
                                            onChange={(e) => updateConfig({schedule: e.target.value})}
                                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            placeholder="0 2 * * *"
                                        />
                                        <p className="mt-1 text-xs text-gray-500">
                                            Leave empty to run immediately
                                        </p>
                                    </div>

                                    <div>
                                        <label className="block text-sm font-medium text-white mb-2">
                                            Environment Variables
                                        </label>
                                        {Object.entries(config.envVars).map(([key, value]) => (
                                            <div key={key} className="flex space-x-2 mb-2">
                                                <input
                                                    type="text"
                                                    value={key}
                                                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md bg-gray-800 text-gray-500 font-medium"
                                                    disabled
                                                />
                                                <input
                                                    type="text"
                                                    value={value}
                                                    onChange={(e) => updateEnvVar(key, e.target.value)}
                                                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                />
                                                <button
                                                    type="button"
                                                    onClick={() => removeEnvVar(key)}
                                                    className="px-3 py-2 text-red-600 hover:text-red-800"
                                                >
                                                    Remove
                                                </button>
                                            </div>
                                        ))}
                                        <div className="flex space-x-2">
                                            <input
                                                type="text"
                                                placeholder="KEY"
                                                className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                onKeyPress={(e) => {
                                                    if (e.key === 'Enter') {
                                                        const keyInput = e.target as HTMLInputElement;
                                                        const valueInput = keyInput.nextElementSibling as HTMLInputElement;
                                                        if (keyInput.value && valueInput.value) {
                                                            updateEnvVar(keyInput.value, valueInput.value);
                                                            keyInput.value = '';
                                                            valueInput.value = '';
                                                        }
                                                    }
                                                }}
                                            />
                                            <input
                                                type="text"
                                                placeholder="value"
                                                className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                onKeyPress={(e) => {
                                                    if (e.key === 'Enter') {
                                                        const valueInput = e.target as HTMLInputElement;
                                                        const keyInput = valueInput.previousElementSibling as HTMLInputElement;
                                                        if (keyInput.value && valueInput.value) {
                                                            updateEnvVar(keyInput.value, valueInput.value);
                                                            keyInput.value = '';
                                                            valueInput.value = '';
                                                        }
                                                    }
                                                }}
                                            />
                                            <button
                                                type="button"
                                                onClick={(e) => {
                                                    const valueInput = e.currentTarget.previousElementSibling as HTMLInputElement;
                                                    const keyInput = valueInput.previousElementSibling as HTMLInputElement;
                                                    if (keyInput.value && valueInput.value) {
                                                        updateEnvVar(keyInput.value, valueInput.value);
                                                        keyInput.value = '';
                                                        valueInput.value = '';
                                                    }
                                                }}
                                                className="px-3 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700"
                                            >
                                                Add
                                            </button>
                                        </div>
                                    </div>

                                    {/* Secret Environment Variables */}
                                    <div>
                                        <label className="block text-sm font-medium text-white mb-2 flex items-center">
                                            <span>Secret Environment Variables</span>
                                            <span
                                                className="ml-2 text-xs text-yellow-400 bg-yellow-900 px-2 py-1 rounded">Hidden from logs</span>
                                        </label>
                                        {Object.entries(config.secretEnvVars).map(([key, value]) => (
                                            <div key={key} className="flex space-x-2 mb-2">
                                                <input
                                                    type="text"
                                                    value={key}
                                                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md bg-gray-100 text-gray-800 font-medium"
                                                    disabled
                                                />
                                                <input
                                                    type="password"
                                                    value={value}
                                                    onChange={(e) => updateSecretEnvVar(key, e.target.value)}
                                                    className="flex-1 px-3 py-2 border border-yellow-500 rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-500 bg-yellow-50"
                                                    placeholder="Hidden value"
                                                />
                                                <button
                                                    type="button"
                                                    onClick={() => removeSecretEnvVar(key)}
                                                    className="px-3 py-2 text-red-600 hover:text-red-800"
                                                >
                                                    Remove
                                                </button>
                                            </div>
                                        ))}
                                        <div className="flex space-x-2">
                                            <input
                                                type="text"
                                                placeholder="SECRET_KEY"
                                                className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                onKeyPress={(e) => {
                                                    if (e.key === 'Enter') {
                                                        const keyInput = e.currentTarget;
                                                        const valueInput = keyInput.nextElementSibling as HTMLInputElement;
                                                        if (keyInput.value && valueInput.value) {
                                                            updateSecretEnvVar(keyInput.value, valueInput.value);
                                                            keyInput.value = '';
                                                            valueInput.value = '';
                                                        }
                                                    }
                                                }}
                                            />
                                            <input
                                                type="password"
                                                placeholder="secret_value"
                                                className="flex-1 px-3 py-2 border border-yellow-500 rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-500 bg-yellow-50"
                                                onKeyPress={(e) => {
                                                    if (e.key === 'Enter') {
                                                        const valueInput = e.currentTarget;
                                                        const keyInput = valueInput.previousElementSibling as HTMLInputElement;
                                                        if (keyInput.value && valueInput.value) {
                                                            updateSecretEnvVar(keyInput.value, valueInput.value);
                                                            keyInput.value = '';
                                                            valueInput.value = '';
                                                        }
                                                    }
                                                }}
                                            />
                                            <button
                                                type="button"
                                                onClick={(e) => {
                                                    const valueInput = e.currentTarget.previousElementSibling as HTMLInputElement;
                                                    const keyInput = valueInput.previousElementSibling as HTMLInputElement;
                                                    if (keyInput.value && valueInput.value) {
                                                        updateSecretEnvVar(keyInput.value, valueInput.value);
                                                        keyInput.value = '';
                                                        valueInput.value = '';
                                                    }
                                                }}
                                                className="px-3 py-2 bg-yellow-600 text-white rounded-md hover:bg-yellow-700"
                                            >
                                                Add Secret
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>

                        {/* Command Preview */}
                        <div>
                            <label className="block text-sm font-medium text-white mb-2">
                                Command Preview
                            </label>
                            <pre
                                className="bg-gray-900 text-green-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
                {preview || '# Configure job options to see command preview'}
              </pre>
                        </div>

                        {/* Actions */}
                        <div className="flex justify-between pt-4 border-t border-gray-200">
                            <div className="space-x-2">
                                <button
                                    type="button"
                                    onClick={resetForm}
                                    className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                                >
                                    <RotateCcw className="w-4 h-4 mr-2"/>
                                    Reset
                                </button>
                            </div>

                            <div className="space-x-2">
                                <button
                                    type="button"
                                    className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                                >
                                    <Save className="w-4 h-4 mr-2"/>
                                    Save Template
                                </button>
                                <button
                                    type="submit"
                                    disabled={loading || !config.command.trim()}
                                    className={clsx(
                                        'inline-flex items-center px-4 py-2 rounded-md text-sm font-medium text-white',
                                        loading || !config.command.trim()
                                            ? 'bg-gray-400 cursor-not-allowed'
                                            : 'bg-blue-600 hover:bg-blue-700'
                                    )}
                                >
                                    <Play className="w-4 h-4 mr-2"/>
                                    {loading ? 'Creating...' : 'Execute Job'}
                                </button>
                            </div>
                        </div>
                    </div>
                </form>
            </div>
        </div>
    );
};