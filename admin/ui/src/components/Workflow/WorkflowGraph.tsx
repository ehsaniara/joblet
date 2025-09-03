import {useCallback, useMemo, useState} from 'react';
import {Job, WorkflowJob} from '../../types/job';
import {JobNode} from './JobNode';
import {Maximize2, RotateCcw, Wifi, WifiOff, ZoomIn, ZoomOut} from 'lucide-react';
import clsx from 'clsx';
import {useWorkflowStatusStream} from '../../hooks/useWorkflowStatusStream';

interface WorkflowGraphProps {
    jobs: WorkflowJob[];
    workflowId?: string;
    onJobSelect?: (job: Job | null) => void;
    onJobAction?: (job: Job, action: string) => void;
    onJobStatusUpdate?: (jobId: string, status: string, updatedJob?: WorkflowJob) => void;
}

interface Position {
    x: number;
    y: number;
}

interface Edge {
    from: string;
    to: string;
    fromPos: Position;
    toPos: Position;
}

const NODE_WIDTH = 140;
const NODE_HEIGHT = 100;
const HORIZONTAL_SPACING = 220;
const VERTICAL_SPACING = 140;

export const WorkflowGraph: React.FC<WorkflowGraphProps> = ({
                                                                jobs,
                                                                workflowId,
                                                                onJobSelect,
                                                                onJobAction,
                                                                onJobStatusUpdate
                                                            }) => {
    const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
    const [zoom, setZoom] = useState(1);
    const [pan, setPan] = useState({x: 0, y: 0});
    const [isPanning, setIsPanning] = useState(false);
    const [lastPanPoint, setLastPanPoint] = useState({x: 0, y: 0});
    const [draggedJobId, setDraggedJobId] = useState<string | null>(null);
    const [jobOverrides, setJobOverrides] = useState<Map<string, Position>>(new Map());
    const [realtimeJobs, setRealtimeJobs] = useState<Map<string, WorkflowJob>>(new Map());
    // const [statusChangeAnimations, setStatusChangeAnimations] = useState<Map<string, number>>(new Map());

    // Handle real-time job status updates
    const handleJobStatusUpdate = useCallback((jobId: string, status: string, updatedJob?: WorkflowJob) => {
        console.log(`Real-time update for job ${jobId}: ${status}`);

        if (updatedJob) {
            setRealtimeJobs(prev => new Map(prev.set(jobId, updatedJob)));
        }

        // TODO: Trigger visual animation for status change
        // setStatusChangeAnimations(prev => new Map(prev.set(jobId, Date.now())));

        // TODO: Clear animation after 2 seconds
        // setTimeout(() => {
        //     setStatusChangeAnimations(prev => {
        //         const newMap = new Map(prev);
        //         newMap.delete(jobId);
        //         return newMap;
        //     });
        // }, 2000);

        // Call parent callback if provided
        if (onJobStatusUpdate) {
            onJobStatusUpdate(jobId, status, updatedJob);
        }
    }, [onJobStatusUpdate]);

    // Connect to real-time workflow status stream
    const {connected, error} = useWorkflowStatusStream(
        workflowId || null,
        handleJobStatusUpdate
    );

    // Merge real-time updates with initial jobs data
    const currentJobs = useMemo(() => {
        return jobs.map(job => {
            const realtimeJob = realtimeJobs.get(job.id);
            return realtimeJob || job;
        });
    }, [jobs, realtimeJobs]);

    // Calculate job positions using a simple layered layout
    const {jobPositions, edges, processedJobs} = useMemo(() => {
        if (currentJobs.length === 0) {
            return {jobPositions: new Map<string, Position>(), edges: [], processedJobs: []};
        }

        // Debug: Log job dependencies and identify potential issues
        console.log('Workflow jobs with dependencies:', currentJobs.map(j => ({
            id: j.id,
            name: j.name,
            dependsOn: j.dependsOn,
            hasValidId: !!(j.id && j.id.toString().length > 0)
        })));

        // Filter and ensure unique IDs for jobs to prevent overlapping
        const seenIds = new Set<string>();
        const validJobs: WorkflowJob[] = [];

        currentJobs.forEach((job, index) => {
            if (job.id && job.id.toString().length > 0) {
                const jobIdStr = job.id.toString();
                if (!seenIds.has(jobIdStr)) {
                    seenIds.add(jobIdStr);
                    validJobs.push(job);
                } else {
                    // Create unique ID for duplicate jobs
                    const uniqueId = `${jobIdStr}-duplicate-${index}`;
                    console.warn(`Duplicate job ID found: ${jobIdStr}, using unique ID: ${uniqueId}`);
                    validJobs.push({
                        ...job,
                        id: uniqueId
                    });
                }
            } else {
                // Create unique ID for jobs with invalid/missing IDs
                const fallbackId = `job-${index}-${Date.now()}`;
                console.warn(`Job with invalid ID found, using fallback ID: ${fallbackId}`);
                validJobs.push({
                    ...job,
                    id: fallbackId
                });
            }
        });

        if (validJobs.length !== jobs.length) {
            console.warn(`Processed ${jobs.length} jobs, resulted in ${validJobs.length} valid jobs`);
        }

        // Create dependency graph
        const dependencies = new Map<string, string[]>();
        const dependents = new Map<string, string[]>();

        validJobs.forEach(job => {
            dependencies.set(job.id, job.dependsOn || []);
            job.dependsOn?.forEach(depId => {
                if (!dependents.has(depId)) {
                    dependents.set(depId, []);
                }
                dependents.get(depId)!.push(job.id);
            });
        });

        // Calculate layers using topological sort
        const layers: string[][] = [];
        const visited = new Set<string>();
        const inDegree = new Map<string, number>();

        // Calculate in-degrees
        validJobs.forEach(job => {
            inDegree.set(job.id, job.dependsOn?.length || 0);
        });

        // Find jobs with no dependencies (layer 0)
        let currentLayer = validJobs.filter(job => (job.dependsOn?.length || 0) === 0).map(job => job.id);

        while (currentLayer.length > 0) {
            layers.push([...currentLayer]);
            const nextLayer: string[] = [];

            currentLayer.forEach(jobId => {
                visited.add(jobId);
                const deps = dependents.get(jobId) || [];
                deps.forEach(depId => {
                    const currentInDegree = inDegree.get(depId) || 0;
                    inDegree.set(depId, currentInDegree - 1);
                    if (inDegree.get(depId) === 0 && !visited.has(depId)) {
                        nextLayer.push(depId);
                    }
                });
            });

            currentLayer = nextLayer;
        }

        // Handle any remaining jobs (cycles or orphans)
        const remaining = validJobs.filter(job => !visited.has(job.id));
        if (remaining.length > 0) {
            layers.push(remaining.map(job => job.id));
        }

        // Calculate positions
        const positions = new Map<string, Position>();

        layers.forEach((layer, layerIndex) => {
            const layerHeight = layer.length * VERTICAL_SPACING;
            const startY = Math.max(50, (600 - layerHeight) / 2); // Center vertically in 600px canvas, with minimum top margin

            layer.forEach((jobId, jobIndex) => {
                positions.set(jobId, {
                    x: layerIndex * HORIZONTAL_SPACING + 50,
                    y: startY + jobIndex * VERTICAL_SPACING + 50
                });
            });
        });

        // Apply any user position overrides
        jobOverrides.forEach((overridePos, jobId) => {
            if (positions.has(jobId)) {
                positions.set(jobId, overridePos);
            }
        });

        // Calculate edges
        const calculatedEdges: Edge[] = [];
        validJobs.forEach(job => {
            const toPos = positions.get(job.id);
            if (!toPos) return;

            job.dependsOn?.forEach(depId => {
                const fromPos = positions.get(depId);
                if (fromPos) {
                    calculatedEdges.push({
                        from: depId,
                        to: job.id,
                        fromPos: {
                            x: fromPos.x + NODE_WIDTH,
                            y: fromPos.y + NODE_HEIGHT / 2
                        },
                        toPos: {
                            x: toPos.x,
                            y: toPos.y + NODE_HEIGHT / 2
                        }
                    });
                }
            });
        });

        // Debug: Log calculated edges
        console.log('Calculated dependency edges:', calculatedEdges);

        return {jobPositions: positions, edges: calculatedEdges, processedJobs: validJobs};
    }, [currentJobs, jobOverrides]);

    const handleJobClick = useCallback((job: Job) => {
        setSelectedJobId(job.id);
        onJobSelect?.(job);
    }, [onJobSelect]);

    const handleJobDoubleClick = useCallback((job: Job) => {
        onJobAction?.(job, 'details');
    }, [onJobAction]);

    const handleZoomIn = useCallback(() => {
        setZoom(prev => Math.min(prev * 1.2, 3));
    }, []);

    const handleZoomOut = useCallback(() => {
        setZoom(prev => Math.max(prev / 1.2, 0.3));
    }, []);

    const handleResetView = useCallback(() => {
        setZoom(1);
        setPan({x: 0, y: 0});
        setSelectedJobId(null);
        setJobOverrides(new Map()); // Reset all position overrides
        onJobSelect?.(null);
    }, [onJobSelect]);

    const handleMouseDown = useCallback((e: React.MouseEvent) => {
        // Allow panning if not dragging a job node
        if (!draggedJobId) {
            setIsPanning(true);
            setLastPanPoint({x: e.clientX, y: e.clientY});
            e.preventDefault();
        }
    }, [draggedJobId]);

    const handleMouseMove = useCallback((e: React.MouseEvent) => {
        if (isPanning) {
            const deltaX = e.clientX - lastPanPoint.x;
            const deltaY = e.clientY - lastPanPoint.y;
            setPan(prev => ({
                x: prev.x + deltaX,
                y: prev.y + deltaY
            }));
            setLastPanPoint({x: e.clientX, y: e.clientY});
        }
    }, [isPanning, lastPanPoint]);

    const handleMouseUp = useCallback(() => {
        setIsPanning(false);
        setDraggedJobId(null);
    }, []);

    const handleJobMouseDown = useCallback((e: React.MouseEvent, jobId: string) => {
        e.stopPropagation();
        setDraggedJobId(jobId);
        setLastPanPoint({
            x: e.clientX,
            y: e.clientY
        });
    }, []);

    const handleJobMouseMove = useCallback((e: React.MouseEvent) => {
        if (draggedJobId) {
            e.stopPropagation();
            const deltaX = (e.clientX - lastPanPoint.x) / zoom;
            const deltaY = (e.clientY - lastPanPoint.y) / zoom;

            const currentPos = jobPositions.get(draggedJobId);
            if (currentPos) {
                const newPos = {
                    x: currentPos.x + deltaX,
                    y: currentPos.y + deltaY
                };

                // Update the override position for this job
                setJobOverrides(prev => {
                    const newOverrides = new Map(prev);
                    newOverrides.set(draggedJobId, newPos);
                    return newOverrides;
                });
            }

            setLastPanPoint({
                x: e.clientX,
                y: e.clientY
            });
        }
    }, [draggedJobId, lastPanPoint, jobPositions, zoom]);

    const handleJobMouseUp = useCallback(() => {
        setDraggedJobId(null);
    }, []);

    // Since validJobs is computed in useMemo, we need to access the selected job differently
    // Use the original jobs array to find selected job for display purposes
    const selectedJob = jobs.find(job => job.id === selectedJobId);

    return (
        <div className="relative w-full h-full bg-gray-900 overflow-hidden">
            {/* Controls */}
            <div className="absolute top-4 right-4 z-20 flex space-x-2">
                <button
                    onClick={handleZoomIn}
                    className="p-2 bg-gray-700 text-white rounded-lg shadow-md hover:bg-gray-600"
                    title="Zoom In">
                    <ZoomIn className="w-4 h-4"/>
                </button>
                <button
                    onClick={handleZoomOut}
                    className="p-2 bg-gray-700 text-white rounded-lg shadow-md hover:bg-gray-600"
                    title="Zoom Out">
                    <ZoomOut className="w-4 h-4"/>
                </button>
                <button
                    onClick={handleResetView}
                    className="p-2 bg-gray-700 text-white rounded-lg shadow-md hover:bg-gray-600"
                    title="Reset View & Positions">
                    <RotateCcw className="w-4 h-4"/>
                </button>
            </div>

            {/* Zoom Level */}
            <div className="absolute top-4 left-4 z-20 bg-gray-700 text-white rounded-lg shadow-md px-3 py-2 text-sm">
                {Math.round(zoom * 100)}%
            </div>

            {/* WebSocket Connection Status */}
            {workflowId && (
                <div
                    className="absolute top-16 left-4 z-20 flex items-center gap-2 bg-gray-800 text-white rounded-lg shadow-md px-3 py-2 text-sm">
                    {connected ? (
                        <>
                            <Wifi className="w-4 h-4 text-green-400"/>
                            <span className="text-green-400 font-medium">Live Updates</span>
                        </>
                    ) : (
                        <>
                            <WifiOff className="w-4 h-4 text-red-400"/>
                            <span className="text-red-400 font-medium">
                                {error ? 'Connection Error' : 'Connecting...'}
                            </span>
                        </>
                    )}
                </div>
            )}

            {/* Canvas */}
            <div
                className={clsx(
                    "w-full h-full relative",
                    isPanning ? "cursor-grabbing" : "cursor-grab"
                )}
                onMouseDown={handleMouseDown}
                onMouseMove={handleMouseMove}
                onMouseUp={handleMouseUp}
                onMouseLeave={handleMouseUp}
            >
                {/* Invisible background for drag interaction */}
                <div className="absolute inset-0 w-full h-full bg-transparent"/>
                <svg
                    className="absolute inset-0 w-full h-full pointer-events-none"
                    style={{
                        transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                        transformOrigin: '0 0'
                    }}>
                    {/* Definitions for patterns and markers */}
                    <defs>
                        <pattern
                            id="grid"
                            width="20"
                            height="20"
                            patternUnits="userSpaceOnUse">
                            <path
                                d="M 20 0 L 0 0 0 20"
                                fill="none"
                                stroke="#374151"
                                strokeWidth="1"
                                opacity="0.5"
                            />
                        </pattern>
                        {/* Single arrow marker definition */}
                        <marker
                            id="arrowhead"
                            markerWidth="8"
                            markerHeight="6"
                            refX="7"
                            refY="3"
                            orient="auto">
                            <polygon
                                points="0 0, 8 3, 0 6"
                                fill="#9ca3af"
                            />
                        </marker>
                    </defs>

                    {/* Grid background */}
                    <rect width="100%" height="100%" fill="url(#grid)"/>

                    {/* Dependency Edges/Arrows */}
                    <g className="dependency-arrows">
                        {edges.map((edge, index) => (
                            <path
                                key={`edge-${edge.from}-${edge.to}-${index}`}
                                d={`M ${edge.fromPos.x} ${edge.fromPos.y} 
                                    C ${edge.fromPos.x + 50} ${edge.fromPos.y} 
                                      ${edge.toPos.x - 50} ${edge.toPos.y} 
                                      ${edge.toPos.x} ${edge.toPos.y}`}
                                fill="none"
                                stroke="#9ca3af"
                                strokeWidth="2.5"
                                strokeDasharray="none"
                                markerEnd="url(#arrowhead)"
                                opacity="0.9"
                            />
                        ))}
                    </g>
                </svg>

                {/* Job Nodes */}
                <div
                    className="absolute inset-0"
                    style={{
                        transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                        transformOrigin: '0 0'
                    }}
                    onMouseMove={handleJobMouseMove}
                    onMouseUp={handleJobMouseUp}>
                    {processedJobs.map((job, index) => {
                        const position = jobPositions.get(job.id);
                        if (!position) return null;

                        return (
                            <JobNode
                                key={`${job.id}-${index}`}
                                job={job}
                                position={position}
                                selected={selectedJobId === job.id}
                                onClick={() => handleJobClick(job)}
                                onDoubleClick={() => handleJobDoubleClick(job)}
                                onMouseDown={(e) => handleJobMouseDown(e, job.id)}
                                isDragging={draggedJobId === job.id}
                            />
                        );
                    })}
                </div>
            </div>

            {/* Empty State */}
            {processedJobs.length === 0 && (
                <div className="absolute inset-0 flex items-center justify-center">
                    <div className="text-center">
                        <Maximize2 className="w-12 h-12 text-gray-500 mx-auto mb-4"/>
                        <h3 className="text-lg font-medium text-white mb-2">
                            No Workflow Jobs
                        </h3>
                        <p className="text-gray-400">
                            Jobs with dependencies will appear as a workflow graph
                        </p>
                    </div>
                </div>
            )}

            {/* Job Details Panel */}
            {selectedJob && (
                <div
                    className="absolute bottom-4 left-4 bg-gray-800 rounded-lg shadow-lg p-4 max-w-sm z-20 border border-gray-700">
                    <h4 className="font-medium text-white mb-2 flex items-center gap-2">
                        <span>{selectedJob.name || selectedJob.id}</span>
                        <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${
                            selectedJob.status === 'RUNNING' ? 'bg-yellow-900 text-yellow-200' :
                                selectedJob.status === 'COMPLETED' ? 'bg-green-900 text-green-200' :
                                    selectedJob.status === 'FAILED' ? 'bg-red-900 text-red-200' :
                                        selectedJob.status === 'CANCELLED' ? 'bg-orange-900 text-orange-200' :
                                            selectedJob.status === 'PENDING' ? 'bg-blue-900 text-blue-200' :
                                                'bg-gray-700 text-gray-200'
                        }`}>
                            {selectedJob.status}
                        </span>
                    </h4>
                    <div className="space-y-1 text-sm text-gray-300">
                        {selectedJob.id && selectedJob.name && (
                            <div>ID: <span className="font-mono text-xs">{selectedJob.id}</span></div>
                        )}

                        <div>Command: <span className="font-mono">{selectedJob.command}</span></div>

                        {selectedJob.args && selectedJob.args.length > 0 && (
                            <div>Args: <span className="font-mono">{selectedJob.args.join(' ')}</span></div>
                        )}

                        {selectedJob.dependsOn && selectedJob.dependsOn.length > 0 && (
                            <div>
                                Dependencies: <span className="font-medium">{selectedJob.dependsOn.length}</span>
                                <div className="ml-2 text-xs">
                                    {selectedJob.dependsOn.map(dep => (
                                        <div key={dep} className="font-mono">→ {dep}</div>
                                    ))}
                                </div>
                            </div>
                        )}

                        {selectedJob.startTime && (
                            <div>Started: <span
                                className="font-medium">{new Date(selectedJob.startTime).toLocaleString()}</span></div>
                        )}

                        {selectedJob.endTime && (
                            <div>Ended: <span
                                className="font-medium">{new Date(selectedJob.endTime).toLocaleString()}</span></div>
                        )}

                        {selectedJob.duration > 0 && (
                            <div>Duration: <span className="font-medium">
                                {selectedJob.duration > 3600000 ?
                                    `${Math.floor(selectedJob.duration / 3600000)}h ${Math.floor((selectedJob.duration % 3600000) / 60000)}m` :
                                    selectedJob.duration > 60000 ?
                                        `${Math.floor(selectedJob.duration / 60000)}m ${Math.floor((selectedJob.duration % 60000) / 1000)}s` :
                                        `${Math.floor(selectedJob.duration / 1000)}s`
                                }
                            </span></div>
                        )}

                        {selectedJob.exitCode !== undefined && selectedJob.exitCode !== null && (
                            <div>Exit Code: <span
                                className={`font-medium ${selectedJob.exitCode === 0 ? 'text-green-600' : 'text-red-600'}`}>
                                {selectedJob.exitCode}
                            </span></div>
                        )}

                        {/* Resource Limits - only show if set */}
                        {(selectedJob.maxCPU > 0 || selectedJob.maxMemory > 0 || selectedJob.maxIOBPS > 0 || selectedJob.cpuCores) && (
                            <div className="mt-2 pt-2 border-t border-gray-600">
                                <div className="text-xs font-medium text-gray-400 mb-1">Resource Limits</div>
                                {selectedJob.maxCPU > 0 && (
                                    <div>Max CPU: <span className="font-medium">{selectedJob.maxCPU}%</span></div>
                                )}
                                {selectedJob.maxMemory > 0 && (
                                    <div>Max Memory: <span className="font-medium">{selectedJob.maxMemory}MB</span>
                                    </div>
                                )}
                                {selectedJob.maxIOBPS > 0 && (
                                    <div>Max IO: <span className="font-medium">{selectedJob.maxIOBPS} BPS</span></div>
                                )}
                                {selectedJob.cpuCores && (
                                    <div>CPU Cores: <span className="font-medium">{selectedJob.cpuCores}</span></div>
                                )}
                            </div>
                        )}

                        {/* Runtime Configuration - only show if set */}
                        {(selectedJob.runtime || selectedJob.network !== 'default' ||
                            (selectedJob.volumes && selectedJob.volumes.length > 0) ||
                            (selectedJob.uploads && selectedJob.uploads.length > 0)) && (
                            <div className="mt-2 pt-2 border-t border-gray-600">
                                <div className="text-xs font-medium text-gray-400 mb-1">Configuration</div>
                                {selectedJob.runtime && (
                                    <div>Runtime: <span className="font-medium">{selectedJob.runtime}</span></div>
                                )}
                                {selectedJob.network && selectedJob.network !== 'default' && (
                                    <div>Network: <span className="font-medium">{selectedJob.network}</span></div>
                                )}
                                {selectedJob.volumes && selectedJob.volumes.length > 0 && (
                                    <div>Volumes: <span className="font-medium">{selectedJob.volumes.length}</span>
                                    </div>
                                )}
                                {selectedJob.uploads && selectedJob.uploads.length > 0 && (
                                    <div>Uploads: <span className="font-medium">{selectedJob.uploads.length}</span>
                                    </div>
                                )}
                            </div>
                        )}

                        {/* Environment Variables - only show if set */}
                        {(Object.keys(selectedJob.envVars || {}).length > 0 ||
                            Object.keys(selectedJob.secretEnvVars || {}).length > 0) && (
                            <div className="mt-2 pt-2 border-t border-gray-600">
                                <div className="text-xs font-medium text-gray-400 mb-1">Environment</div>
                                {Object.keys(selectedJob.envVars || {}).length > 0 && (
                                    <div>Variables: <span
                                        className="font-medium">{Object.keys(selectedJob.envVars || {}).length}</span>
                                    </div>
                                )}
                                {Object.keys(selectedJob.secretEnvVars || {}).length > 0 && (
                                    <div>Secrets: <span
                                        className="font-medium">{Object.keys(selectedJob.secretEnvVars || {}).length}</span>
                                    </div>
                                )}
                            </div>
                        )}

                        {/* Resource Usage - only show if available */}
                        {selectedJob.resourceUsage && (
                            <div className="mt-2 pt-2 border-t border-gray-600">
                                <div className="text-xs font-medium text-gray-400 mb-1">Current Usage</div>
                                <div>CPU: <span
                                    className="font-medium">{Math.round(selectedJob.resourceUsage.cpuPercent)}%</span>
                                </div>
                                <div>Memory: <span
                                    className="font-medium">{Math.round(selectedJob.resourceUsage.memoryPercent)}%</span>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
};