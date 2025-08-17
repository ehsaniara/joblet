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

export interface DetailedSystemInfo {
    hostInfo: {
        hostname?: string;
        platform?: string;
        arch?: string;
        release?: string;
        uptime?: number;
        cloudProvider?: string;
        instanceType?: string;
        region?: string;
    };
    cpuInfo: {
        cores?: number;
        threads?: number;
        model?: string;
        frequency?: number;
        usage?: number;
        loadAverage?: number[];
        perCoreUsage?: number[];
        temperature?: number;
    };
    memoryInfo: {
        total?: number;
        used?: number;
        available?: number;
        percent?: number;
        buffers?: number;
        cached?: number;
        swap?: {
            total: number;
            used: number;
            percent: number;
        };
    };
    disksInfo: {
        disks?: Array<{
            name: string;
            mountpoint: string;
            filesystem: string;
            size: number;
            used: number;
            available: number;
            percent: number;
            readBps?: number;
            writeBps?: number;
            iops?: number;
        }>;
        totalSpace?: number;
        usedSpace?: number;
    };
    networkInfo: {
        interfaces?: Array<{
            name: string;
            type: string;
            status: string;
            speed?: number;
            mtu?: number;
            ipAddresses?: string[];
            macAddress?: string;
            rxBytes?: number;
            txBytes?: number;
            rxPackets?: number;
            txPackets?: number;
            rxErrors?: number;
            txErrors?: number;
        }>;
        totalRxBytes?: number;
        totalTxBytes?: number;
    };
    processesInfo: {
        processes?: Array<{
            pid: number;
            name: string;
            command: string;
            user: string;
            cpu: number;
            memory: number;
            memoryBytes: number;
            status: string;
            startTime?: string;
            threads?: number;
        }>;
        totalProcesses?: number;
    };
}