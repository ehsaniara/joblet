export interface WorkflowTemplate {
    version: string;
    name: string;
    description?: string;
    jobs: Record<string, WorkflowJob>;
}

export interface WorkflowJob {
    command: string[];
    dependsOn?: string[];
    uploads?: {
        files?: string[];
        directories?: string[];
    };
    resources?: {
        maxCpu?: number;
        maxMemory?: number;
        maxIobps?: number;
        cpuCores?: string;
    };
    runtime?: string;
    network?: string;
    volumes?: string[];
    envVars?: Record<string, string>;
    workdir?: string;
    retries?: number;
    timeout?: string;
}