import React from 'react';
import { Cpu } from 'lucide-react';

interface CPUDetailsCardProps {
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
}

const CPUDetailsCard: React.FC<CPUDetailsCardProps> = ({ cpuInfo }) => {
    return (
        <div className="bg-gray-800 rounded-lg shadow p-6">
            <div className="flex items-center mb-4">
                <Cpu className="h-6 w-6 text-blue-600 mr-3"/>
                <h3 className="text-lg font-semibold text-white">CPU Details</h3>
            </div>
            
            <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                    <div>
                        <span className="text-sm text-gray-400">Model</span>
                        <div className="font-medium text-white">{cpuInfo.model || 'Unknown'}</div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Frequency</span>
                        <div className="font-medium text-white">
                            {cpuInfo.frequency ? `${cpuInfo.frequency} MHz` : 'Unknown'}
                        </div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Cores</span>
                        <div className="font-medium text-white">{cpuInfo.cores || 0}</div>
                    </div>
                    <div>
                        <span className="text-sm text-gray-400">Threads</span>
                        <div className="font-medium text-white">{cpuInfo.threads || cpuInfo.cores || 0}</div>
                    </div>
                </div>

                <div>
                    <div className="flex justify-between mb-2">
                        <span className="text-sm text-gray-400">Overall Usage</span>
                        <span className="text-sm font-medium text-white">
                            {cpuInfo.usage ? `${cpuInfo.usage.toFixed(1)}%` : '0%'}
                        </span>
                    </div>
                    <div className="w-full bg-gray-700 rounded-full h-3">
                        <div
                            className="bg-blue-600 h-3 rounded-full transition-all duration-300"
                            style={{width: `${cpuInfo.usage || 0}%`}}
                        ></div>
                    </div>
                </div>

                {cpuInfo.loadAverage && (
                    <div>
                        <span className="text-sm text-gray-400">Load Average (1m, 5m, 15m)</span>
                        <div className="font-medium text-white">
                            {cpuInfo.loadAverage.map(load => load.toFixed(2)).join(', ')}
                        </div>
                    </div>
                )}

                {cpuInfo.temperature && (
                    <div>
                        <span className="text-sm text-gray-400">Temperature</span>
                        <div className="font-medium text-white">{cpuInfo.temperature}°C</div>
                    </div>
                )}

                {cpuInfo.perCoreUsage && cpuInfo.perCoreUsage.length > 0 && (
                    <div>
                        <span className="text-sm text-gray-400 mb-2 block">Per-Core Usage</span>
                        <div className="grid grid-cols-4 gap-2">
                            {cpuInfo.perCoreUsage.map((usage, index) => (
                                <div key={index} className="text-center">
                                    <div className="text-xs text-gray-400">Core {index}</div>
                                    <div className="w-full bg-gray-700 rounded-full h-2 mt-1">
                                        <div
                                            className="bg-blue-500 h-2 rounded-full"
                                            style={{width: `${usage}%`}}
                                        ></div>
                                    </div>
                                    <div className="text-xs text-white mt-1">{usage.toFixed(0)}%</div>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default CPUDetailsCard;