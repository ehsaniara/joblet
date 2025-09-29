export interface SystemMetrics {
    timestamp: string;
    available: boolean;
    cpu: {
        cores: number;
        usagePercent: number;
        loadAverage: number[];
        perCoreUsage: number[];
    };
    memory: {
        totalBytes: number;
        usedBytes: number;
        availableBytes: number;
        usagePercent: number;
        cachedBytes: number;
        bufferedBytes: number;
    };
    disks: Array<{
        device: string;
        mountPoint: string;
        filesystem: string;
        totalBytes: number;
        usedBytes: number;
        freeBytes: number;
        usagePercent: number;
    }>;
    processes: {
        totalProcesses: number;
        totalThreads: number;
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
        nodeId?: string;
        serverIPs?: string[];
        macAddresses?: string[];
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
    gpuInfo?: {
        gpus?: Array<{
            id: number;
            name: string;
            memoryTotal: number;
            memoryUsed: number;
            memoryFree: number;
            utilizationGpu: number;
            utilizationMemory: number;
            temperature: number;
            powerDraw: number;
            powerLimit: number;
            status: string;
        }>;
        totalGpus?: number;
        cudaVersion?: string;
        driverVersion?: string;
    };
}