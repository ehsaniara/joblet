import React, { useState } from 'react';
import { List, Search, MoreVertical } from 'lucide-react';

interface ProcessesCardProps {
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

const ProcessesCard: React.FC<ProcessesCardProps> = ({ processesInfo }) => {
    const [searchTerm, setSearchTerm] = useState('');
    const [sortBy, setSortBy] = useState<'cpu' | 'memory' | 'name'>('cpu');
    const [showCount, setShowCount] = useState(10);

    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    };

    const getStatusColor = (status: string) => {
        switch (status.toLowerCase()) {
            case 'running':
                return 'text-green-500';
            case 'sleeping':
                return 'text-blue-500';
            case 'stopped':
                return 'text-red-500';
            case 'zombie':
                return 'text-yellow-500';
            default:
                return 'text-gray-400';
        }
    };

    const filteredAndSortedProcesses = processesInfo.processes
        ? processesInfo.processes
            .filter(process => 
                process.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
                process.command.toLowerCase().includes(searchTerm.toLowerCase()) ||
                process.user.toLowerCase().includes(searchTerm.toLowerCase())
            )
            .sort((a, b) => {
                switch (sortBy) {
                    case 'cpu':
                        return b.cpu - a.cpu;
                    case 'memory':
                        return b.memory - a.memory;
                    case 'name':
                        return a.name.localeCompare(b.name);
                    default:
                        return 0;
                }
            })
            .slice(0, showCount)
        : [];

    return (
        <div className="bg-gray-800 rounded-lg shadow p-6">
            <div className="flex items-center justify-between mb-4">
                <div className="flex items-center">
                    <List className="h-6 w-6 text-orange-600 mr-3"/>
                    <h3 className="text-lg font-semibold text-white">
                        Running Processes
                        {processesInfo.totalProcesses && (
                            <span className="text-gray-400 ml-2">
                                ({processesInfo.totalProcesses} total)
                            </span>
                        )}
                    </h3>
                </div>
                <div className="flex items-center space-x-2">
                    <select
                        value={sortBy}
                        onChange={(e) => setSortBy(e.target.value as 'cpu' | 'memory' | 'name')}
                        className="bg-gray-700 text-white border border-gray-600 rounded px-2 py-1 text-sm"
                    >
                        <option value="cpu">Sort by CPU</option>
                        <option value="memory">Sort by Memory</option>
                        <option value="name">Sort by Name</option>
                    </select>
                    <select
                        value={showCount}
                        onChange={(e) => setShowCount(Number(e.target.value))}
                        className="bg-gray-700 text-white border border-gray-600 rounded px-2 py-1 text-sm"
                    >
                        <option value={10}>Top 10</option>
                        <option value={25}>Top 25</option>
                        <option value={50}>Top 50</option>
                    </select>
                </div>
            </div>

            <div className="mb-4">
                <div className="relative">
                    <Search className="h-4 w-4 text-gray-400 absolute left-3 top-1/2 transform -translate-y-1/2"/>
                    <input
                        type="text"
                        placeholder="Search processes..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="w-full bg-gray-700 text-white border border-gray-600 rounded-lg pl-10 pr-4 py-2 text-sm"
                    />
                </div>
            </div>
            
            <div className="space-y-2">
                {filteredAndSortedProcesses.length > 0 ? (
                    <>
                        <div className="grid grid-cols-12 gap-2 text-xs text-gray-400 font-medium border-b border-gray-700 pb-2">
                            <div className="col-span-1">PID</div>
                            <div className="col-span-3">Name</div>
                            <div className="col-span-2">User</div>
                            <div className="col-span-2">CPU %</div>
                            <div className="col-span-2">Memory</div>
                            <div className="col-span-1">Status</div>
                            <div className="col-span-1">Threads</div>
                        </div>
                        {filteredAndSortedProcesses.map((process, index) => (
                            <div key={index} className="grid grid-cols-12 gap-2 text-sm py-2 hover:bg-gray-700 rounded px-2">
                                <div className="col-span-1 text-gray-300">{process.pid}</div>
                                <div className="col-span-3">
                                    <div className="text-white font-medium truncate" title={process.name}>
                                        {process.name}
                                    </div>
                                    <div className="text-gray-400 text-xs truncate" title={process.command}>
                                        {process.command}
                                    </div>
                                </div>
                                <div className="col-span-2 text-gray-300">{process.user}</div>
                                <div className="col-span-2">
                                    <div className="flex items-center">
                                        <div className="w-8 bg-gray-600 rounded-full h-1 mr-2">
                                            <div
                                                className="bg-orange-500 h-1 rounded-full"
                                                style={{width: `${Math.min(process.cpu, 100)}%`}}
                                            ></div>
                                        </div>
                                        <span className="text-white text-xs">
                                            {process.cpu.toFixed(1)}%
                                        </span>
                                    </div>
                                </div>
                                <div className="col-span-2">
                                    <div className="text-white text-xs">
                                        {formatBytes(process.memoryBytes)}
                                    </div>
                                    <div className="text-gray-400 text-xs">
                                        {process.memory.toFixed(1)}%
                                    </div>
                                </div>
                                <div className={`col-span-1 text-xs ${getStatusColor(process.status)}`}>
                                    {process.status}
                                </div>
                                <div className="col-span-1 text-gray-300 text-xs">
                                    {process.threads || '-'}
                                </div>
                            </div>
                        ))}
                    </>
                ) : (
                    <div className="text-center py-8">
                        <List className="h-12 w-12 text-gray-400 mx-auto mb-4"/>
                        <p className="text-gray-400">
                            {searchTerm ? 'No processes match your search' : 'No process information available'}
                        </p>
                    </div>
                )}
            </div>
        </div>
    );
};

export default ProcessesCard;