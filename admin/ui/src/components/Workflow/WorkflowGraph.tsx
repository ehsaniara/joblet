import React, {useCallback, useMemo, useState} from 'react';
import {Job} from '../../types/job';
import {JobNode} from './JobNode';
import {Maximize2, RotateCcw, ZoomIn, ZoomOut} from 'lucide-react';

interface WorkflowGraphProps {
    jobs: Job[];
    onJobSelect?: (job: Job | null) => void;
    onJobAction?: (job: Job, action: string) => void;
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

const NODE_WIDTH = 160;
const NODE_HEIGHT = 120;
const HORIZONTAL_SPACING = 200;
const VERTICAL_SPACING = 150;

export const WorkflowGraph: React.FC<WorkflowGraphProps> = ({
                                                                jobs,
                                                                onJobSelect,
                                                                onJobAction
                                                            }) => {
    const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
    const [zoom, setZoom] = useState(1);
    const [pan, setPan] = useState({x: 0, y: 0});
    const [isPanning, setIsPanning] = useState(false);
    const [lastPanPoint, setLastPanPoint] = useState({x: 0, y: 0});

    // Calculate job positions using a simple layered layout
    const {jobPositions, edges} = useMemo(() => {
        if (jobs.length === 0) {
            return {jobPositions: new Map<string, Position>(), edges: []};
        }

        // Create dependency graph
        const dependencies = new Map<string, string[]>();
        const dependents = new Map<string, string[]>();

        jobs.forEach(job => {
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
        jobs.forEach(job => {
            inDegree.set(job.id, job.dependsOn?.length || 0);
        });

        // Find jobs with no dependencies (layer 0)
        let currentLayer = jobs.filter(job => (job.dependsOn?.length || 0) === 0).map(job => job.id);

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
        const remaining = jobs.filter(job => !visited.has(job.id));
        if (remaining.length > 0) {
            layers.push(remaining.map(job => job.id));
        }

        // Calculate positions
        const positions = new Map<string, Position>();

        layers.forEach((layer, layerIndex) => {
            const layerHeight = layer.length * VERTICAL_SPACING;
            const startY = (600 - layerHeight) / 2; // Center vertically in 600px canvas

            layer.forEach((jobId, jobIndex) => {
                positions.set(jobId, {
                    x: layerIndex * HORIZONTAL_SPACING + 50,
                    y: startY + jobIndex * VERTICAL_SPACING + 50
                });
            });
        });

        // Calculate edges
        const calculatedEdges: Edge[] = [];
        jobs.forEach(job => {
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

        return {jobPositions: positions, edges: calculatedEdges};
    }, [jobs]);

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
        onJobSelect?.(null);
    }, [onJobSelect]);

    const handleMouseDown = useCallback((e: React.MouseEvent) => {
        if (e.target === e.currentTarget) {
            setIsPanning(true);
            setLastPanPoint({x: e.clientX, y: e.clientY});
        }
    }, []);

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
    }, []);

    const selectedJob = jobs.find(job => job.id === selectedJobId);

    return (
        <div className="relative w-full h-full bg-gray-50 overflow-hidden">
            {/* Controls */}
            <div className="absolute top-4 right-4 z-20 flex space-x-2">
                <button
                    onClick={handleZoomIn}
                    className="p-2 bg-white rounded-lg shadow-md hover:bg-gray-50"
                    title="Zoom In"
                >
                    <ZoomIn className="w-4 h-4"/>
                </button>
                <button
                    onClick={handleZoomOut}
                    className="p-2 bg-white rounded-lg shadow-md hover:bg-gray-50"
                    title="Zoom Out"
                >
                    <ZoomOut className="w-4 h-4"/>
                </button>
                <button
                    onClick={handleResetView}
                    className="p-2 bg-white rounded-lg shadow-md hover:bg-gray-50"
                    title="Reset View"
                >
                    <RotateCcw className="w-4 h-4"/>
                </button>
            </div>

            {/* Zoom Level */}
            <div className="absolute top-4 left-4 z-20 bg-white rounded-lg shadow-md px-3 py-2 text-sm">
                {Math.round(zoom * 100)}%
            </div>

            {/* Canvas */}
            <div
                className="w-full h-full cursor-grab active:cursor-grabbing"
                onMouseDown={handleMouseDown}
                onMouseMove={handleMouseMove}
                onMouseUp={handleMouseUp}
                onMouseLeave={handleMouseUp}
            >
                <svg
                    className="absolute inset-0 w-full h-full"
                    style={{
                        transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                        transformOrigin: '0 0'
                    }}
                >
                    {/* Grid Pattern */}
                    <defs>
                        <pattern
                            id="grid"
                            width="20"
                            height="20"
                            patternUnits="userSpaceOnUse"
                        >
                            <path
                                d="M 20 0 L 0 0 0 20"
                                fill="none"
                                stroke="#e5e7eb"
                                strokeWidth="1"
                                opacity="0.5"
                            />
                        </pattern>
                    </defs>
                    <rect width="100%" height="100%" fill="url(#grid)"/>

                    {/* Edges */}
                    {edges.map((edge, index) => (
                        <g key={`edge-${edge.from}-${edge.to}-${index}`}>
                            <path
                                d={`M ${edge.fromPos.x} ${edge.fromPos.y} 
                   C ${edge.fromPos.x + 50} ${edge.fromPos.y} 
                     ${edge.toPos.x - 50} ${edge.toPos.y} 
                     ${edge.toPos.x} ${edge.toPos.y}`}
                                fill="none"
                                stroke="#9ca3af"
                                strokeWidth="2"
                                markerEnd="url(#arrowhead)"
                            />
                            {/* Arrow marker */}
                            <defs>
                                <marker
                                    id="arrowhead"
                                    markerWidth="10"
                                    markerHeight="7"
                                    refX="9"
                                    refY="3.5"
                                    orient="auto"
                                >
                                    <polygon
                                        points="0 0, 10 3.5, 0 7"
                                        fill="#9ca3af"
                                    />
                                </marker>
                            </defs>
                        </g>
                    ))}
                </svg>

                {/* Job Nodes */}
                <div
                    className="absolute inset-0"
                    style={{
                        transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                        transformOrigin: '0 0'
                    }}
                >
                    {jobs.map(job => {
                        const position = jobPositions.get(job.id);
                        if (!position) return null;

                        return (
                            <JobNode
                                key={job.id}
                                job={job}
                                position={position}
                                selected={selectedJobId === job.id}
                                onClick={() => handleJobClick(job)}
                                onDoubleClick={() => handleJobDoubleClick(job)}
                            />
                        );
                    })}
                </div>
            </div>

            {/* Empty State */}
            {jobs.length === 0 && (
                <div className="absolute inset-0 flex items-center justify-center">
                    <div className="text-center">
                        <Maximize2 className="w-12 h-12 text-gray-400 mx-auto mb-4"/>
                        <h3 className="text-lg font-medium text-gray-900 mb-2">
                            No Workflow Jobs
                        </h3>
                        <p className="text-gray-500">
                            Jobs with dependencies will appear as a workflow graph
                        </p>
                    </div>
                </div>
            )}

            {/* Job Details Panel */}
            {selectedJob && (
                <div className="absolute bottom-4 left-4 bg-white rounded-lg shadow-lg p-4 max-w-sm z-20">
                    <h4 className="font-medium text-gray-900 mb-2">
                        {selectedJob.name || selectedJob.id}
                    </h4>
                    <div className="space-y-1 text-sm text-gray-600">
                        <div>Status: <span className="font-medium">{selectedJob.status}</span></div>
                        <div>Command: <span className="font-mono">{selectedJob.command}</span></div>
                        {selectedJob.dependsOn && selectedJob.dependsOn.length > 0 && (
                            <div>
                                Dependencies: <span className="font-medium">{selectedJob.dependsOn.length}</span>
                            </div>
                        )}
                        {selectedJob.resourceUsage && (
                            <div className="mt-2 pt-2 border-t">
                                <div>CPU: {Math.round(selectedJob.resourceUsage.cpuPercent)}%</div>
                                <div>Memory: {Math.round(selectedJob.resourceUsage.memoryPercent)}%</div>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
};