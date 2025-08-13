export interface APIResponse<T> {
    data: T;
    error?: string;
    timestamp: string;
}

export interface JobListResponse {
    jobs: Job[];
    total: number;
}


import {Job} from './job';