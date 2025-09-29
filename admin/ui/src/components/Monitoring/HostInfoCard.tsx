// React import not needed with modern JSX transform
import {Server} from 'lucide-react';

interface HostInfoCardProps {
    hostInfo: {
        hostname?: string;
        platform?: string;
        arch?: string;
        kernel?: string;
        uptime?: number;
        timezone?: string;
        cloudProvider?: string;
        region?: string;
        instanceType?: string;
        nodeId?: string;
        serverIPs?: string[];
        macAddresses?: string[];
    };
}

const HostInfoCard: React.FC<HostInfoCardProps> = ({hostInfo}) => {
    const formatUptime = (seconds: number) => {
        const days = Math.floor(seconds / 86400);
        const hours = Math.floor((seconds % 86400) / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);

        if (days > 0) {
            return `${days}d ${hours}h ${minutes}m`;
        } else if (hours > 0) {
            return `${hours}h ${minutes}m`;
        } else {
            return `${minutes}m`;
        }
    };

    return (
        <div className="bg-gray-800 rounded-lg shadow p-6">
            <div className="flex items-center mb-4">
                <Server className="h-6 w-6 text-blue-600 mr-3"/>
                <h3 className="text-lg font-semibold text-white">Host Information</h3>
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="space-y-3">
                    {hostInfo.nodeId && (
                        <div>
                            <span className="text-sm text-gray-400">Node ID</span>
                            <div className="font-medium text-white font-mono">{hostInfo.nodeId}</div>
                        </div>
                    )}
                    <div>
                        <span className="text-sm text-gray-400">Hostname</span>
                        <div className="font-medium text-white">{hostInfo.hostname || 'Unknown'}</div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Platform</span>
                        <div className="font-medium text-white">{hostInfo.platform || 'Unknown'}</div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Architecture</span>
                        <div className="font-medium text-white">{hostInfo.arch || 'Unknown'}</div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Kernel</span>
                        <div className="font-medium text-white">{hostInfo.kernel || 'Unknown'}</div>
                    </div>
                </div>

                <div className="space-y-3">
                    <div>
                        <span className="text-sm text-gray-400">Uptime</span>
                        <div className="font-medium text-white">
                            {hostInfo.uptime ? formatUptime(hostInfo.uptime) : 'Unknown'}
                        </div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Timezone</span>
                        <div className="font-medium text-white">{hostInfo.timezone || 'Unknown'}</div>
                    </div>
                    {hostInfo.cloudProvider && (
                        <>
                            <div>
                                <span className="text-sm text-gray-400">Cloud Provider</span>
                                <div className="font-medium text-white">{hostInfo.cloudProvider}</div>
                            </div>
                            <div>
                                <span className="text-sm text-gray-400">Region</span>
                                <div className="font-medium text-white">{hostInfo.region || 'Unknown'}</div>
                            </div>
                        </>
                    )}
                </div>
            </div>
        </div>
    );
};

export default HostInfoCard;