// React import not needed with modern JSX transform
import {Database} from 'lucide-react';

interface Volume {
    id?: string;
    name: string;
    size: string;
    type: string;
    created_time?: string;
    mountPath?: string;
}

interface VolumesCardProps {
    volumes: Volume[];
}

const VolumesCard: React.FC<VolumesCardProps> = ({volumes}) => {
    const formatSize = (size: string) => {
        return size;
    };

    const formatDate = (dateStr: string) => {
        try {
            return new Date(dateStr).toLocaleDateString();
        } catch {
            return dateStr;
        }
    };

    const getSizeInBytes = (size: string) => {
        const match = size.match(/(\d+(?:\.\d+)?)\s*(GB|MB|KB|B)/i);
        if (!match) return 0;

        const value = parseFloat(match[1]);
        const unit = match[2].toUpperCase();

        switch (unit) {
            case 'GB':
                return value * 1024 * 1024 * 1024;
            case 'MB':
                return value * 1024 * 1024;
            case 'KB':
                return value * 1024;
            default:
                return value;
        }
    };

    const getTotalSize = () => {
        const totalBytes = volumes.reduce((total, volume) => total + getSizeInBytes(volume.size), 0);
        const gb = totalBytes / (1024 * 1024 * 1024);
        return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(totalBytes / (1024 * 1024)).toFixed(0)} MB`;
    };

    const getSizeDistribution = () => {
        const distribution: Record<string, number> = {};
        volumes.forEach(volume => {
            distribution[volume.size] = (distribution[volume.size] || 0) + 1;
        });
        return distribution;
    };

    const getTypeColor = (type: string) => {
        switch (type) {
            case 'filesystem':
                return 'text-blue-400';
            case 'block':
                return 'text-green-400';
            default:
                return 'text-gray-400';
        }
    };

    return (
        <div className="bg-gray-800 rounded-lg shadow p-6">
            <div className="flex items-center mb-4">
                <Database className="h-6 w-6 text-blue-600 mr-3"/>
                <h3 className="text-lg font-semibold text-white">Volume Information</h3>
            </div>

            <div className="space-y-4">
                {volumes && volumes.length > 0 ? (
                    <>
                        {/* Summary Stats */}
                        <div className="grid grid-cols-2 gap-4 p-4 bg-gray-700 rounded-lg">
                            <div>
                                <span className="text-gray-400 text-sm">Total Volumes</span>
                                <div className="font-medium text-white text-lg">{volumes.length}</div>
                            </div>
                            <div>
                                <span className="text-gray-400 text-sm">Total Allocated</span>
                                <div className="font-medium text-white text-lg">{getTotalSize()}</div>
                            </div>
                        </div>

                        {/* Size Distribution */}
                        <div className="p-4 bg-gray-700 rounded-lg">
                            <h4 className="text-sm font-medium text-gray-300 mb-3">Size Distribution</h4>
                            <div className="grid grid-cols-2 gap-2 text-sm">
                                {Object.entries(getSizeDistribution()).map(([size, count]) => (
                                    <div key={size} className="flex justify-between">
                                        <span className="text-gray-400">{size}</span>
                                        <span className="text-white">{count} volume{count !== 1 ? 's' : ''}</span>
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Volume List */}
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                            {volumes.map((volume, index) => (
                                <div key={index} className="border border-gray-700 rounded-lg p-4">
                                    <div className="mb-3">
                                        <div className="font-medium text-white text-sm truncate" title={volume.name}>
                                            {volume.name}
                                        </div>
                                        <div className="flex justify-between items-center mt-1">
                                            <div className={`text-xs ${getTypeColor(volume.type)}`}>
                                                {volume.type}
                                            </div>
                                            <div className="text-sm font-medium text-white">
                                                {formatSize(volume.size)}
                                            </div>
                                        </div>
                                        {volume.created_time && (
                                            <div className="text-xs text-gray-400 mt-1">
                                                {formatDate(volume.created_time)}
                                            </div>
                                        )}
                                    </div>

                                    {/* Volume usage indicator - for now just visual, no actual usage data */}
                                    <div className="w-full bg-gray-700 rounded-full h-1.5">
                                        <div
                                            className="h-1.5 rounded-full bg-blue-600 transition-all duration-300"
                                            style={{width: '0%'}} // No usage data available from API
                                        ></div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </>
                ) : (
                    <div className="text-center py-8">
                        <Database className="h-12 w-12 text-gray-400 mx-auto mb-4"/>
                        <p className="text-gray-400">No volumes found</p>
                    </div>
                )}
            </div>
        </div>
    );
};

export default VolumesCard;