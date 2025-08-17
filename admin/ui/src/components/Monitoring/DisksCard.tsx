import React from 'react';
import { HardDrive } from 'lucide-react';

interface DisksCardProps {
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
}

const DisksCard: React.FC<DisksCardProps> = ({ disksInfo }) => {
    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    };

    const formatBps = (bps: number) => {
        return formatBytes(bps) + '/s';
    };

    return (
        <div className="bg-gray-800 rounded-lg shadow p-6">
            <div className="flex items-center mb-4">
                <HardDrive className="h-6 w-6 text-purple-600 mr-3"/>
                <h3 className="text-lg font-semibold text-white">Disk Information</h3>
            </div>
            
            <div className="space-y-4">
                {disksInfo.disks && disksInfo.disks.length > 0 ? (
                    disksInfo.disks.map((disk, index) => (
                        <div key={index} className="border border-gray-700 rounded-lg p-4">
                            <div className="flex justify-between items-center mb-3">
                                <div>
                                    <div className="font-medium text-white">{disk.name}</div>
                                    <div className="text-sm text-gray-400">{disk.mountpoint}</div>
                                    <div className="text-xs text-gray-500">{disk.filesystem}</div>
                                </div>
                                <div className="text-right">
                                    <div className="text-sm font-medium text-white">
                                        {formatBytes(disk.used)} / {formatBytes(disk.size)}
                                    </div>
                                    <div className="text-sm text-gray-400">
                                        {disk.percent.toFixed(1)}% used
                                    </div>
                                </div>
                            </div>
                            
                            <div className="w-full bg-gray-700 rounded-full h-2 mb-3">
                                <div
                                    className={`h-2 rounded-full transition-all duration-300 ${
                                        disk.percent > 90 ? 'bg-red-600' :
                                        disk.percent > 75 ? 'bg-yellow-600' : 'bg-purple-600'
                                    }`}
                                    style={{width: `${Math.min(disk.percent, 100)}%`}}
                                ></div>
                            </div>

                            {(disk.readBps !== undefined || disk.writeBps !== undefined || disk.iops !== undefined) && (
                                <div className="grid grid-cols-3 gap-4 text-sm">
                                    <div>
                                        <span className="text-gray-400">Read</span>
                                        <div className="font-medium text-white">
                                            {disk.readBps ? formatBps(disk.readBps) : '0 B/s'}
                                        </div>
                                    </div>
                                    <div>
                                        <span className="text-gray-400">Write</span>
                                        <div className="font-medium text-white">
                                            {disk.writeBps ? formatBps(disk.writeBps) : '0 B/s'}
                                        </div>
                                    </div>
                                    <div>
                                        <span className="text-gray-400">IOPS</span>
                                        <div className="font-medium text-white">
                                            {disk.iops || 0}
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    ))
                ) : (
                    <div className="text-center py-8">
                        <HardDrive className="h-12 w-12 text-gray-400 mx-auto mb-4"/>
                        <p className="text-gray-400">No disk information available</p>
                    </div>
                )}
            </div>
        </div>
    );
};

export default DisksCard;