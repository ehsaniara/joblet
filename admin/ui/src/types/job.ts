export type JobStatus =
    | 'INITIALIZING'
    | 'RUNNING'
    | 'COMPLETED'
    | 'FAILED'
    | 'STOPPED'
    | 'QUEUED'
    | 'WAITING';

export interface Job {
    id: string;
    name: string;
    command: string;
    args: string[];
    status: JobStatus;
    startTime: string;
    endTime?: string;
    duration: number;
    exitCode?: number;
    maxCPU: number;
    maxMemory: number;
    maxIOBPS: number;
    cpuCores?: string;
    runtime?: string;
    network: string;
    volumes: string[];
    uploads: string[];
    uploadDirs: string[];
    envVars: Record<string, string>;
    workdir: string;
    dependsOn: string[];
    resourceUsage?: ResourceUsage;
}

export interface ResourceUsage {
    cpuPercent: number;
    memoryUsed: number;
    memoryPercent: number;
    ioRead: number;
    ioWrite: number;
    diskUsed: number;
}

export interface JobConfig {
    command: string;
    files: string[]; // File paths from upload service
    directories: string[]; // Directory paths from upload service
    maxCpu: number;
    maxMemory: number;
    cpuCores: string;
    maxIobps: number;
    runtime: string;
    network: string;
    volumes: string[];
    envVars: Record<string, string>;
    schedule: string;
    name: string;
    workdir: string;
}

export interface JobExecuteRequest {
    command: string;
    args?: string[];
    maxCPU?: number;
    maxMemory?: number;
    maxIOBPS?: number;
    cpuCores?: string;
    runtime?: string;
    network?: string;
    volumes?: string[];
    uploads?: string[];
    uploadDirs?: string[];
    envVars?: Record<string, string>;
    schedule?: string;
    name?: string;
    workdir?: string;
}

export interface JobFlag {
    flag: string;
    value: string | number | boolean;
    multiple?: boolean;
}

export interface GeneratedCommand {
    command: string;
    flags: JobFlag[];
    fullCommand: string;
}