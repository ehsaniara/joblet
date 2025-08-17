import React from 'react';
import { HardDrive } from 'lucide-react';

interface MemoryDetailsCardProps {
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
}

const MemoryDetailsCard: React.FC<MemoryDetailsCardProps> = ({ memoryInfo }) => {
    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    };

    return (
        <div className="bg-gray-800 rounded-lg shadow p-6">
            <div className="flex items-center mb-4">
                <HardDrive className="h-6 w-6 text-green-600 mr-3"/>
                <h3 className="text-lg font-semibold text-white">Memory Details</h3>
            </div>
            
            <div className="space-y-4">
                <div>
                    <div className="flex justify-between mb-2">
                        <span className="text-sm text-gray-400">Memory Usage</span>
                        <span className="text-sm font-medium text-white">
                            {formatBytes(memoryInfo.used || 0)} / {formatBytes(memoryInfo.total || 0)}
                            {memoryInfo.percent && ` (${memoryInfo.percent.toFixed(1)}%)`}
                        </span>
                    </div>
                    <div className="w-full bg-gray-700 rounded-full h-3">
                        <div
                            className="bg-green-600 h-3 rounded-full transition-all duration-300"
                            style={{width: `${memoryInfo.percent || 0}%`}}
                        ></div>
                    </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                    <div>
                        <span className="text-sm text-gray-400">Available</span>
                        <div className="font-medium text-white">
                            {formatBytes(memoryInfo.available || 0)}
                        </div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Total</span>
                        <div className="font-medium text-white">
                            {formatBytes(memoryInfo.total || 0)}
                        </div>
                    </div>
                </div>

                {(memoryInfo.buffers || memoryInfo.cached) && (
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <span className="text-sm text-gray-400">Buffers</span>
                            <div className="font-medium text-white">
                                {formatBytes(memoryInfo.buffers || 0)}
                            </div>
                        </div>
                        <div>
                            <span className="text-sm text-gray-400">Cached</span>
                            <div className="font-medium text-white">
                                {formatBytes(memoryInfo.cached || 0)}
                            </div>
                        </div>
                    </div>
                )}

                {memoryInfo.swap && (
                    <div className="border-t border-gray-700 pt-4">
                        <div className="flex justify-between mb-2">
                            <span className="text-sm text-gray-400">Swap Usage</span>
                            <span className="text-sm font-medium text-white">
                                {formatBytes(memoryInfo.swap.used)} / {formatBytes(memoryInfo.swap.total)}
                                {` (${memoryInfo.swap.percent.toFixed(1)}%)`}
                            </span>
                        </div>
                        <div className="w-full bg-gray-700 rounded-full h-2">
                            <div
                                className="bg-yellow-600 h-2 rounded-full transition-all duration-300"
                                style={{width: `${memoryInfo.swap.percent}%`}}
                            ></div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default MemoryDetailsCard;