import {useMemo, useState} from 'react';
import {WorkflowJob} from '@/types';
import {AlertCircle, CheckCircle, Clock, FileText, Loader2, XCircle} from 'lucide-react';
import clsx from 'clsx';

interface WorkflowTimelineProps {
    jobs: WorkflowJob[];
    onJobClick?: (jobId: string) => void;
}

interface TimelineJob {
    id: string;
    name?: string;
    command: string;
    args?: string[];
    status: string;
    dependsOn?: string[];
    startTime?: Date;
    endTime?: Date;
    duration?: number;
    relativeStart?: number;
    relativeEnd?: number;
    level?: number;
}

const WorkflowTimeline: React.FC<WorkflowTimelineProps> = ({jobs, onJobClick}) => {
    const [hoveredJob, setHoveredJob] = useState<string | null>(null);
    const [selectedTimeRange, setSelectedTimeRange] = useState<'all' | '1min' | '5min' | '30min'>('all');

    const processedJobs = useMemo(() => {
        const timelineJobs: TimelineJob[] = jobs.map(job => {
            const startTime = job.startTime ? new Date(job.startTime) : undefined;
            const endTime = job.endTime ? new Date(job.endTime) : undefined;
            const duration = startTime && endTime ? endTime.getTime() - startTime.getTime() : 0;

            return {
                ...job,
                startTime,
                endTime,
                duration
            };
        });

        // Find valid jobs with start times
        const validJobs = timelineJobs.filter(j => j.startTime);
        if (validJobs.length === 0) return [];

        // Calculate relative positions and assign levels for sequential visualization
        // For workflow execution, prefer sequential layout over parallel to show logical flow
        const sortedJobs = [...validJobs].sort((a, b) => a.startTime!.getTime() - b.startTime!.getTime());

        return sortedJobs.map((job, index) => {
            // Create artificial sequential spacing for better visualization
            // Distribute jobs evenly across the timeline regardless of actual timing
            const totalJobs = sortedJobs.length;
            const jobWidth = Math.max(15, 80 / totalJobs); // Each job gets at least 15% width
            const relativeStart = index * (100 / totalJobs);
            const relativeEnd = Math.min(100, relativeStart + jobWidth);

            // For workflow visualization, assign each job to its own level
            // This provides clear sequential view like a thread profiler
            const level = index;

            return {
                ...job,
                relativeStart,
                relativeEnd,
                level
            };
        });
    }, [jobs, selectedTimeRange]);

    const getStatusColor = (status: string) => {
        switch (status?.toUpperCase()) {
            case 'RUNNING':
                return 'bg-yellow-500';
            case 'COMPLETED':
                return 'bg-green-500';
            case 'FAILED':
                return 'bg-red-500';
            case 'STOPPED':
                return 'bg-gray-500';
            case 'QUEUED':
            case 'PENDING':
                return 'bg-blue-500';
            case 'CANCELLED':
                return 'bg-orange-500';
            default:
                return 'bg-gray-400';
        }
    };

    const getStatusIcon = (status: string) => {
        switch (status?.toUpperCase()) {
            case 'RUNNING':
                return <Loader2 className="h-4 w-4 animate-spin"/>;
            case 'COMPLETED':
                return <CheckCircle className="h-4 w-4"/>;
            case 'FAILED':
                return <XCircle className="h-4 w-4"/>;
            case 'STOPPED':
            case 'CANCELLED':
                return <AlertCircle className="h-4 w-4"/>;
            case 'QUEUED':
            case 'PENDING':
                return <Clock className="h-4 w-4"/>;
            default:
                return <Clock className="h-4 w-4"/>;
        }
    };

    const formatDuration = (ms: number) => {
        const seconds = Math.floor(ms / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);

        if (hours > 0) {
            return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
        } else if (minutes > 0) {
            return `${minutes}m ${seconds % 60}s`;
        } else {
            return `${seconds}s`;
        }
    };

    const formatTime = (date: Date) => {
        return date.toLocaleTimeString('en-US', {
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false
        });
    };

    // Generate time markers
    const timeMarkers = useMemo(() => {
        if (processedJobs.length === 0) return [];

        const validJobs = processedJobs.filter(j => j.startTime);
        if (validJobs.length === 0) return [];

        const minTime = Math.min(...validJobs.map(j => j.startTime!.getTime()));
        const maxTime = Math.max(...validJobs.map(j => j.endTime?.getTime() || j.startTime!.getTime()));

        // Apply time range filter
        let filteredMaxTime = maxTime;
        if (selectedTimeRange !== 'all') {
            const rangeMinutes = selectedTimeRange === '1min' ? 1 : selectedTimeRange === '5min' ? 5 : 30;
            const rangeMs = rangeMinutes * 60 * 1000;
            filteredMaxTime = Math.min(maxTime, minTime + rangeMs);
        }

        const duration = filteredMaxTime - minTime;
        const markers = [];

        // Determine appropriate interval based on duration
        let interval;
        if (duration < 60000) { // Less than 1 minute
            interval = 10000; // 10 seconds
        } else if (duration < 300000) { // Less than 5 minutes
            interval = 30000; // 30 seconds
        } else if (duration < 1800000) { // Less than 30 minutes
            interval = 300000; // 5 minutes
        } else {
            interval = 600000; // 10 minutes
        }

        for (let time = minTime; time <= filteredMaxTime; time += interval) {
            const position = ((time - minTime) / duration) * 100;
            markers.push({
                time: new Date(time),
                position: Math.min(100, position)
            });
        }

        return markers;
    }, [processedJobs, selectedTimeRange]);

    // Calculate the maximum level for height calculation
    const maxLevel = Math.max(0, ...processedJobs.map(j => j.level || 0));
    const timelineHeight = 150 + (maxLevel + 1) * 60;

    if (jobs.length === 0) {
        return (
            <div className="p-4 h-full">
                <div className="bg-gray-800 rounded-lg shadow h-full flex flex-col">
                    <div className="p-4 border-b border-gray-700">
                        <div className="flex items-center justify-between">
                            <h3 className="text-lg font-medium text-white">Workflow Timeline</h3>
                        </div>
                    </div>

                    <div className="flex-1 p-4 flex items-center justify-center">
                        <div className="text-center">
                            <FileText className="h-8 w-8 text-gray-400 mx-auto mb-2"/>
                            <p className="text-gray-400">No timeline data available</p>
                            <p className="text-sm text-gray-500 mt-1">Jobs will appear here once the workflow starts
                                executing</p>
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    const hasStartedJobs = processedJobs.length > 0;

    return (
        <div className="p-4 h-full">
            <div className="bg-gray-800 rounded-lg shadow h-full flex flex-col">
                <div className="p-4 border-b border-gray-700">
                    <div className="flex items-center justify-between">
                        <h3 className="text-lg font-medium text-white">Workflow Timeline</h3>
                        {hasStartedJobs && (
                            <div className="flex items-center space-x-2">
                                <span className="text-sm text-gray-400">Time Range:</span>
                                <select
                                    value={selectedTimeRange}
                                    onChange={(e) => setSelectedTimeRange(e.target.value as any)}
                                    className="px-3 py-1 text-sm bg-gray-700 text-white border border-gray-600 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
                                >
                                    <option value="all">All</option>
                                    <option value="1min">First 1 min</option>
                                    <option value="5min">First 5 min</option>
                                    <option value="30min">First 30 min</option>
                                </select>
                            </div>
                        )}
                    </div>
                </div>

                <div className="flex-1 p-4 overflow-hidden">{/* Changed from p-6 to p-4 and added overflow-hidden */}

                    {!hasStartedJobs ? (
                        <div className="text-center py-8">
                            <Clock className="h-8 w-8 text-gray-400 mx-auto mb-2"/>
                            <p className="text-gray-400">No jobs have started executing yet</p>
                            <p className="text-sm text-gray-500 mt-1">Timeline will be generated once jobs begin</p>
                        </div>
                    ) : (
                        <div className="relative h-full flex flex-col">
                            {/* Timeline Container */}
                            <div
                                className="relative bg-gray-900 rounded-lg p-4 overflow-auto flex-1"
                                style={{
                                    minHeight: `${Math.max(timelineHeight, 400)}px`,
                                    maxHeight: 'calc(100vh - 500px)'
                                }}
                            >
                                {/* Time markers */}
                                <div className="absolute inset-x-4 top-0 h-full">
                                    {timeMarkers.map((marker, index) => (
                                        <div
                                            key={index}
                                            className="absolute top-0 h-full border-l border-gray-700"
                                            style={{left: `${marker.position}%`}}
                                        >
                                            <span
                                                className="absolute -top-6 -left-12 text-xs text-gray-500 whitespace-nowrap">
                                                {formatTime(marker.time)}
                                            </span>
                                        </div>
                                    ))}
                                </div>

                                {/* Job bars */}
                                <div className="relative mt-8" style={{height: `${timelineHeight - 70}px`}}>
                                    {processedJobs.map((job) => {
                                        const width = Math.max(5, (job.relativeEnd || 0) - (job.relativeStart || 0));
                                        const isHovered = hoveredJob === job.id;
                                        const topPosition = (job.level || 0) * 60;

                                        return (
                                            <div
                                                key={job.id}
                                                className="absolute h-10 cursor-pointer transition-all duration-200"
                                                style={{
                                                    left: `${job.relativeStart}%`,
                                                    width: `${width}%`,
                                                    top: `${topPosition}px`,
                                                    zIndex: isHovered ? 10 : 1
                                                }}
                                                onMouseEnter={() => setHoveredJob(job.id)}
                                                onMouseLeave={() => setHoveredJob(null)}
                                                onClick={() => onJobClick?.(job.id)}
                                            >
                                                <div
                                                    className={clsx(
                                                        'h-full rounded flex items-center px-2 shadow-lg transition-all duration-200',
                                                        getStatusColor(job.status),
                                                        isHovered && 'ring-2 ring-white ring-opacity-50 transform scale-105'
                                                    )}
                                                >
                                                    <div
                                                        className="flex items-center space-x-1 text-white overflow-hidden">
                                                        {getStatusIcon(job.status)}
                                                        <span className="text-xs font-medium truncate">
                                                            {job.name || job.id}
                                                        </span>
                                                    </div>
                                                </div>

                                                {/* Tooltip */}
                                                {isHovered && (
                                                    <div
                                                        className={`absolute left-0 z-20 bg-gray-700 text-white p-3 rounded shadow-lg text-xs whitespace-nowrap ${
                                                            (job.level || 0) <= 1 ? 'top-full mt-2' : 'bottom-full mb-2'
                                                        }`}>
                                                        <div className="font-semibold">{job.name || job.id}</div>
                                                        <div className="mt-1">Status: {job.status}</div>
                                                        {job.startTime && (
                                                            <div>Start: {formatTime(job.startTime)}</div>
                                                        )}
                                                        {job.endTime && (
                                                            <div>End: {formatTime(job.endTime)}</div>
                                                        )}
                                                        {job.duration && job.duration > 0 && (
                                                            <div>Duration: {formatDuration(job.duration)}</div>
                                                        )}
                                                        {job.dependsOn && job.dependsOn.length > 0 && (
                                                            <div className="mt-1 pt-1 border-t border-gray-600">
                                                                Depends on: {job.dependsOn.join(', ')}
                                                            </div>
                                                        )}
                                                    </div>
                                                )}
                                            </div>
                                        );
                                    })}
                                </div>
                            </div>

                            {/* Legend */}
                            <div className="mt-4 flex flex-wrap items-center gap-4 text-xs">
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-green-500 rounded"></div>
                                    <span className="text-gray-400">Completed</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-yellow-500 rounded"></div>
                                    <span className="text-gray-400">Running</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-red-500 rounded"></div>
                                    <span className="text-gray-400">Failed</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-blue-500 rounded"></div>
                                    <span className="text-gray-400">Queued/Pending</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-orange-500 rounded"></div>
                                    <span className="text-gray-400">Cancelled</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-gray-500 rounded"></div>
                                    <span className="text-gray-400">Stopped</span>
                                </div>
                            </div>

                            {/* Summary Statistics */}
                            {hasStartedJobs && (
                                <div className="mt-6 grid grid-cols-1 md:grid-cols-4 gap-4">
                                    <div className="bg-gray-700 rounded p-3">
                                        <div className="text-xs text-gray-400">Total Jobs</div>
                                        <div className="text-lg font-semibold text-white">{jobs.length}</div>
                                    </div>
                                    <div className="bg-gray-700 rounded p-3">
                                        <div className="text-xs text-gray-400">Completed</div>
                                        <div className="text-lg font-semibold text-green-400">
                                            {jobs.filter(j => j.status === 'COMPLETED').length}
                                        </div>
                                    </div>
                                    <div className="bg-gray-700 rounded p-3">
                                        <div className="text-xs text-gray-400">Failed</div>
                                        <div className="text-lg font-semibold text-red-400">
                                            {jobs.filter(j => j.status === 'FAILED').length}
                                        </div>
                                    </div>
                                    <div className="bg-gray-700 rounded p-3">
                                        <div className="text-xs text-gray-400">Total Duration</div>
                                        <div className="text-lg font-semibold text-white">
                                            {(() => {
                                                const validJobs = processedJobs.filter(j => j.startTime);
                                                if (validJobs.length === 0) return '-';

                                                // Calculate workflow execution time: first job start to last job end
                                                const startTimes = validJobs.map(j => new Date(j.startTime!).getTime());
                                                const endTimes = validJobs.map(j => j.endTime ? new Date(j.endTime).getTime() : new Date(j.startTime!).getTime());

                                                const workflowStart = Math.min(...startTimes);
                                                const workflowEnd = Math.max(...endTimes);

                                                return formatDuration(workflowEnd - workflowStart);
                                            })()}
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default WorkflowTimeline;