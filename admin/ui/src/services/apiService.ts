import {DetailedSystemInfo, Job, JobExecuteRequest, SystemMetrics, WorkflowTemplate} from '../types';

interface Volume {
    id?: string;
    name: string;
    size: string;
    type: string;
    created_time?: string;
    mountPath?: string;
}

interface Network {
    id: string;
    name: string;
    type: string;
    subnet: string;
}

interface Runtime {
    id: string;
    name: string;
    version: string;
    size: string;
    description: string;
}

interface Node {
    name: string;
    status: string;
}

export const API_BASE_URL = (import.meta as any).env?.DEV
    ? 'http://localhost:5173'
    : '';

class APIService {
    private baseURL: string;
    private currentNode = 'default';

    constructor() {
        this.baseURL = `${API_BASE_URL}/api`;
    }

    setNode(node: string) {
        this.currentNode = node;
    }

    // Node operations
    async getNodes(): Promise<Node[]> {
        return this.request<Node[]>('/nodes', {}, false); // Don't add node param for this call
    }

    // Job operations
    async getJobs(): Promise<Job[]> {
        return this.request<Job[]>('/jobs');
    }

    // Workflow operations
    async getWorkflows(): Promise<any[]> {
        return this.request<any[]>('/workflows');
    }

    async getWorkflow(workflowId: string): Promise<any> {
        return this.request<any>(`/workflows/${workflowId}`);
    }

    async browseWorkflowDirectory(path?: string): Promise<{
        currentPath: string;
        parentPath: string | null;
        directories: Array<{ name: string; path: string; type: string }>;
        yamlFiles: Array<{ name: string; path: string; type: string; selectable: boolean }>;
        otherFiles: Array<{ name: string; path: string; type: string; selectable: boolean }>;
    }> {
        const url = path ? `/workflows/browse?path=${encodeURIComponent(path)}` : '/workflows/browse';
        return this.request<{
            currentPath: string;
            parentPath: string | null;
            directories: Array<{ name: string; path: string; type: string }>;
            yamlFiles: Array<{ name: string; path: string; type: string; selectable: boolean }>;
            otherFiles: Array<{ name: string; path: string; type: string; selectable: boolean }>;
        }>(url, {}, false); // Don't add node param for browsing
    }

    async validateWorkflow(filePath: string): Promise<{
        valid: boolean;
        requiredVolumes: string[];
        missingVolumes: string[];
        message: string;
    }> {
        return this.request<{
            valid: boolean;
            requiredVolumes: string[];
            missingVolumes: string[];
            message: string;
        }>('/workflows/validate', {
            method: 'POST',
            body: JSON.stringify({filePath}),
        });
    }

    async executeWorkflow(filePath: string, workflowName?: string, createMissingVolumes = false): Promise<{
        workflowId: string;
        status: string;
        message: string;
        availableWorkflows?: string[];
        requiresWorkflowSelection?: boolean;
    }> {
        return this.request<{
            workflowId: string;
            status: string;
            message: string;
            availableWorkflows?: string[];
            requiresWorkflowSelection?: boolean;
        }>('/workflows/execute', {
            method: 'POST',
            body: JSON.stringify({filePath, workflowName, createMissingVolumes}),
        });
    }

    async getJob(jobId: string): Promise<Job> {
        return this.request<Job>(`/jobs/${jobId}`);
    }

    async executeJob(request: JobExecuteRequest): Promise<{ jobId: string }> {
        return this.request<{ jobId: string }>('/jobs/execute', {
            method: 'POST',
            body: JSON.stringify({...request, node: this.currentNode}),
        });
    }

    async stopJob(jobId: string): Promise<void> {
        await this.request(`/jobs/${jobId}/stop`, {
            method: 'POST',
            body: JSON.stringify({node: this.currentNode}),
        });
    }

    async deleteJob(jobId: string): Promise<void> {
        await this.request(`/jobs/${jobId}?node=${this.currentNode}`, {
            method: 'DELETE',
        });
    }

    async getJobLogs(jobId: string): Promise<{ logs: string[] }> {
        return this.request<{ logs: string[] }>(`/jobs/${jobId}/logs`);
    }

    // System monitoring
    async getSystemMetrics(): Promise<SystemMetrics> {
        return this.request<SystemMetrics>('/monitor');
    }

    async getDetailedSystemInfo(): Promise<DetailedSystemInfo> {
        return this.request<DetailedSystemInfo>('/system-info');
    }

    // Volume operations
    async getVolumes(): Promise<{ volumes: Volume[] }> {
        return this.request<{ volumes: Volume[] }>('/volumes');
    }

    async deleteVolume(volumeName: string): Promise<{ success: boolean; message: string }> {
        return this.request<{ success: boolean; message: string }>(`/volumes/${encodeURIComponent(volumeName)}`, {
            method: 'DELETE',
        });
    }

    async createVolume(name: string, size: string, type = 'filesystem'): Promise<{
        success: boolean;
        message: string
    }> {
        return this.request<{ success: boolean; message: string }>('/volumes', {
            method: 'POST',
            body: JSON.stringify({name, size, type}),
        });
    }

    // Network operations
    async getNetworks(): Promise<{ networks: Network[] }> {
        return this.request<{ networks: Network[] }>('/networks');
    }

    async createNetwork(name: string, cidr: string): Promise<{ success: boolean; message: string }> {
        return this.request<{ success: boolean; message: string }>('/networks', {
            method: 'POST',
            body: JSON.stringify({name, cidr}),
        });
    }

    async deleteNetwork(networkName: string): Promise<{ success: boolean; message: string }> {
        return this.request<{ success: boolean; message: string }>(`/networks/${encodeURIComponent(networkName)}`, {
            method: 'DELETE',
        });
    }

    // Runtime operations
    async getRuntimes(): Promise<{ runtimes: Runtime[] }> {
        return this.request<{ runtimes: Runtime[] }>('/runtimes');
    }

    // Template operations
    async executeTemplate(template: WorkflowTemplate): Promise<{ workflowId: string }> {
        return this.request<{ workflowId: string }>('/template/execute', {
            method: 'POST',
            body: JSON.stringify(template),
        });
    }

    async validateTemplate(template: WorkflowTemplate): Promise<{ valid: boolean; errors?: string[] }> {
        return this.request<{ valid: boolean; errors?: string[] }>('/template/validate', {
            method: 'POST',
            body: JSON.stringify(template),
        });
    }

    private async request<T>(
        endpoint: string,
        options: RequestInit = {},
        includeNode = true
    ): Promise<T> {
        // Add node parameter to GET requests
        let url = `${this.baseURL}${endpoint}`;
        if (includeNode && (!options.method || options.method === 'GET')) {
            const separator = url.includes('?') ? '&' : '?';
            url = `${url}${separator}node=${encodeURIComponent(this.currentNode)}`;
        }

        const response = await fetch(url, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers,
            },
            ...options,
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API Error: ${response.status} - ${errorText}`);
        }

        return response.json();
    }
}

export const apiService = new APIService();