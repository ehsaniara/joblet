export interface SystemMetrics {
    timestamp: string;
    cpu: {
        cores: number;
        usage: number;
        loadAverage: number[];
    };
    memory: {
        total: number;
        used: number;
        available: number;
        percent: number;
    };
    disk: {
        readBps: number;
        writeBps: number;
        iops: number;
    };
    jobs: {
        total: number;
        running: number;
        completed: number;
        failed: number;
    };
}