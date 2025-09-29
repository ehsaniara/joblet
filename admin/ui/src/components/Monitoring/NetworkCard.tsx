import {Activity, Network, Wifi} from 'lucide-react';

interface NetworkCardProps {
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
}

const NetworkCard: React.FC<NetworkCardProps> = ({networkInfo}) => {
    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    };

    const getInterfaceIcon = (type: string) => {
        switch (type.toLowerCase()) {
            case 'wireless':
            case 'wifi':
                return <Wifi className="h-4 w-4"/>;
            default:
                return <Network className="h-4 w-4"/>;
        }
    };

    const getStatusColor = (status: string) => {
        switch (status.toLowerCase()) {
            case 'up':
            case 'active':
                return 'text-green-500';
            case 'down':
            case 'inactive':
                return 'text-red-500';
            default:
                return 'text-yellow-500';
        }
    };

    return (
        <div className="bg-gray-800 rounded-lg shadow p-6">
            <div className="flex items-center mb-4">
                <Network className="h-6 w-6 text-indigo-600 mr-3"/>
                <h3 className="text-lg font-semibold text-white">Network Interfaces</h3>
            </div>


            <div className="space-y-4">
                {networkInfo.interfaces && networkInfo.interfaces.length > 0 ? (
                    networkInfo.interfaces.map((iface, index) => (
                        <div key={index} className="border border-gray-700 rounded-lg p-4">
                            <div className="flex justify-between items-start mb-3">
                                <div className="flex items-center space-x-2">
                                    {getInterfaceIcon(iface.type)}
                                    <div>
                                        <div className="font-medium text-white">{iface.name}</div>
                                        <div className="text-sm text-gray-400">{iface.type}</div>
                                    </div>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <Activity className={`h-4 w-4 ${getStatusColor(iface.status)}`}/>
                                    <span className={`text-sm font-medium ${getStatusColor(iface.status)}`}>
                                        {iface.status}
                                    </span>
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-4 text-sm">
                                <div>
                                    <span className="text-gray-400">IP Addresses</span>
                                    <div className="font-medium text-white">
                                        {iface.ipAddresses && iface.ipAddresses.length > 0
                                            ? iface.ipAddresses.join(', ')
                                            : 'None'}
                                    </div>
                                </div>
                                <div>
                                    <span className="text-gray-400">MAC Address</span>
                                    <div className="font-medium text-white">
                                        {iface.macAddress || 'Unknown'}
                                    </div>
                                </div>
                                {iface.speed && (
                                    <>
                                        <div>
                                            <span className="text-gray-400">Speed</span>
                                            <div className="font-medium text-white">
                                                {iface.speed >= 1000
                                                    ? `${(iface.speed / 1000).toFixed(1)} Gbps`
                                                    : `${iface.speed} Mbps`}
                                            </div>
                                        </div>
                                        <div>
                                            <span className="text-gray-400">MTU</span>
                                            <div className="font-medium text-white">
                                                {iface.mtu || 'Unknown'}
                                            </div>
                                        </div>
                                    </>
                                )}
                            </div>

                            {(iface.rxBytes !== undefined || iface.txBytes !== undefined) && (
                                <div className="mt-3 pt-3 border-t border-gray-700">
                                    <div className="grid grid-cols-2 gap-4 text-sm">
                                        <div>
                                            <span className="text-gray-400">RX (Received)</span>
                                            <div className="font-medium text-white">
                                                {formatBytes(iface.rxBytes || 0)}
                                                {iface.rxPackets && (
                                                    <span className="text-gray-400 ml-2">
                                                        ({iface.rxPackets.toLocaleString()} packets)
                                                    </span>
                                                )}
                                            </div>
                                            {iface.rxErrors && iface.rxErrors > 0 && (
                                                <div className="text-red-400 text-xs">
                                                    {iface.rxErrors} errors
                                                </div>
                                            )}
                                        </div>
                                        <div>
                                            <span className="text-gray-400">TX (Transmitted)</span>
                                            <div className="font-medium text-white">
                                                {formatBytes(iface.txBytes || 0)}
                                                {iface.txPackets && (
                                                    <span className="text-gray-400 ml-2">
                                                        ({iface.txPackets.toLocaleString()} packets)
                                                    </span>
                                                )}
                                            </div>
                                            {iface.txErrors && iface.txErrors > 0 && (
                                                <div className="text-red-400 text-xs">
                                                    {iface.txErrors} errors
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    ))
                ) : (
                    <div className="text-center py-8">
                        <Network className="h-12 w-12 text-gray-400 mx-auto mb-4"/>
                        <p className="text-gray-400">No network interface information available</p>
                    </div>
                )}
            </div>
        </div>
    );
};

export default NetworkCard;