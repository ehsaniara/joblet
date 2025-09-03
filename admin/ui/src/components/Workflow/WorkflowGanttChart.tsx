import React, {useEffect, useRef} from 'react';
import mermaid from 'mermaid';
import {WorkflowJob} from '@/types';
import {FileText} from 'lucide-react';

interface WorkflowGanttChartProps {
    jobs: WorkflowJob[];
    onJobClick?: (jobId: string) => void;
}

const WorkflowGanttChart: React.FC<WorkflowGanttChartProps> = ({
                                                                   jobs,
                                                                   onJobClick
                                                               }) => {
    const ganttRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        // Initialize Mermaid with dark theme for Gantt
        mermaid.initialize({
            startOnLoad: true,
            theme: 'dark',
            themeVariables: {
                primaryColor: '#1f2937',
                primaryTextColor: '#f9fafb',
                primaryBorderColor: '#374151',
                lineColor: '#6b7280',
                secondaryColor: '#374151',
                tertiaryColor: '#111827',
                background: '#1f2937',
                cScale0: '#065f46', // Completed - dark green
                cScale1: '#92400e', // Running - dark orange  
                cScale2: '#991b1b', // Failed - dark red
                cScale3: '#1e40af', // Pending - dark blue
                cScale4: '#6b7280', // Stopped - gray
            },
            gantt: {
                useMaxWidth: true,
                leftPadding: 120,
                gridLineStartPadding: 35,
                fontSize: 12,
                sectionFontSize: 16,
                numberSectionStyles: 4,
                topPadding: 50,
            }
        });
    }, []);

    useEffect(() => {
        if (!ganttRef.current || jobs.length === 0) return;

        // Generate Mermaid Gantt chart syntax
        const generateGanttCode = () => {
            // Filter jobs that have start times and sort by start time
            const validJobs = jobs.filter(job => job.startTime);
            if (validJobs.length === 0) {
                return `gantt
    title Workflow Execution Timeline
    dateFormat YYYY-MM-DD
    section No Data
    No jobs with timing data : milestone, 2024-01-01, 0d`;
            }

            const sortedJobs = [...validJobs].sort((a, b) =>
                new Date(a.startTime!).getTime() - new Date(b.startTime!).getTime()
            );

            // Use a simpler approach - single section with all jobs
            let code = 'gantt\n';
            code += '    title Workflow Execution Timeline\n';
            code += '    dateFormat YYYY-MM-DD HH:mm:ss\n';
            code += '    axisFormat %H:%M:%S\n';
            code += '    section Jobs\n';

            sortedJobs.forEach((job, index) => {
                const jobName = (job.name || job.id).replace(/[^a-zA-Z0-9\s]/g, ''); // Sanitize job names

                // Force different start times to avoid overlapping by adding index-based offset
                const baseStartTime = new Date(job.startTime!).getTime();
                const adjustedStartTime = new Date(baseStartTime + (index * 2000)); // Add 2 seconds per job
                const adjustedEndTime = job.endTime ?
                    new Date(new Date(job.endTime).getTime() + (index * 2000)) :
                    new Date(adjustedStartTime.getTime() + 1000); // Default 1 second duration

                const startTime = adjustedStartTime.toISOString().replace('T', ' ').replace('Z', '');
                const endTime = adjustedEndTime.toISOString().replace('T', ' ').replace('Z', '');

                // Determine task status for styling
                let taskStatus = '';
                switch (job.status?.toUpperCase()) {
                    case 'COMPLETED':
                        taskStatus = 'done';
                        break;
                    case 'RUNNING':
                        taskStatus = 'active';
                        break;
                    case 'FAILED':
                        taskStatus = 'crit';
                        break;
                    default:
                        taskStatus = '';
                }

                // Add the task
                code += `    ${jobName} :${taskStatus}, ${startTime}, ${endTime}\n`;
            });

            return code;
        };

        const renderGantt = async () => {
            try {
                const code = generateGanttCode();
                console.log('Gantt code:', code);

                // Clear previous content
                ganttRef.current!.innerHTML = '';

                // Generate unique ID for this diagram
                const diagramId = `gantt-${Date.now()}`;

                // Render the diagram
                const {svg} = await mermaid.render(diagramId, code);
                ganttRef.current!.innerHTML = svg;

                // Add click handlers to task bars - use a simpler approach
                const validJobs = jobs.filter(job => job.startTime).sort((a, b) =>
                    new Date(a.startTime!).getTime() - new Date(b.startTime!).getTime()
                );

                const taskElements = ganttRef.current!.querySelectorAll('.task');
                taskElements.forEach((task, index) => {
                    if (index < validJobs.length) {
                        const job = validJobs[index];

                        const handleClick = () => {
                            console.log('Gantt task clicked:', job.name || job.id);
                            if (onJobClick) {
                                onJobClick(job.id);
                            }
                        };

                        // Remove existing listeners to avoid duplicates
                        task.removeEventListener('click', handleClick);
                        task.addEventListener('click', handleClick);

                        // Add hover effects
                        task.addEventListener('mouseenter', () => {
                            task.setAttribute('style', 'cursor: pointer; opacity: 0.8;');
                        });

                        task.addEventListener('mouseleave', () => {
                            task.setAttribute('style', 'cursor: pointer; opacity: 1;');
                        });

                        // Initial styling
                        task.setAttribute('style', 'cursor: pointer;');
                    }
                });

            } catch (error) {
                console.error('Error rendering Gantt chart:', error);
                ganttRef.current!.innerHTML = `
                    <div class="flex items-center justify-center h-64 text-gray-400">
                        <div class="text-center">
                            <p>Error rendering timeline chart</p>
                            <p class="text-sm mt-2">${error instanceof Error ? error.message : 'Unknown error'}</p>
                        </div>
                    </div>
                `;
            }
        };

        renderGantt();
    }, [jobs, onJobClick]);

    if (jobs.length === 0) {
        return (
            <div className="p-6">
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <div className="text-center py-8">
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

    const hasStartedJobs = jobs.some(job => job.startTime);

    return (
        <div className="p-4 h-full">
            <div className="bg-gray-800 rounded-lg shadow h-full flex flex-col">
                <div className="p-4 border-b border-gray-700">
                    <div className="flex items-center justify-between">
                        <h3 className="text-lg font-medium text-white">Workflow Timeline</h3>
                    </div>
                </div>

                <div className="flex-1 p-4">
                    {!hasStartedJobs ? (
                        <div className="text-center py-8">
                            <FileText className="h-8 w-8 text-gray-400 mx-auto mb-2"/>
                            <p className="text-gray-400">No jobs have started executing yet</p>
                            <p className="text-sm text-gray-500 mt-1">Timeline will be generated once jobs begin</p>
                        </div>
                    ) : (
                        <div className="relative h-full flex flex-col">
                            {/* Gantt Chart Container */}
                            <div
                                ref={ganttRef}
                                className="flex-1 w-full bg-gray-900 rounded-lg p-4 overflow-auto"
                                style={{minHeight: '500px', height: 'calc(100vh - 400px)'}}
                            />

                            {/* Legend */}
                            <div className="mt-4 flex flex-wrap items-center gap-4 text-xs">
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-green-600 rounded"></div>
                                    <span className="text-gray-400">Completed</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-orange-600 rounded"></div>
                                    <span className="text-gray-400">Running</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-red-600 rounded"></div>
                                    <span className="text-gray-400">Failed</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-blue-600 rounded"></div>
                                    <span className="text-gray-400">Pending</span>
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="w-3 h-3 bg-gray-600 rounded"></div>
                                    <span className="text-gray-400">Other</span>
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
                                                const validJobs = jobs.filter(j => j.startTime);
                                                if (validJobs.length === 0) return '-';

                                                // Calculate workflow execution time: first job start to last job end
                                                const startTimes = validJobs.map(j => new Date(j.startTime!).getTime());
                                                const endTimes = validJobs.map(j => j.endTime ? new Date(j.endTime).getTime() : new Date(j.startTime!).getTime());

                                                const workflowStart = Math.min(...startTimes);
                                                const workflowEnd = Math.max(...endTimes);

                                                const totalMs = workflowEnd - workflowStart;
                                                const seconds = Math.floor(totalMs / 1000);
                                                const minutes = Math.floor(seconds / 60);
                                                const hours = Math.floor(minutes / 60);

                                                if (hours > 0) {
                                                    return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
                                                } else if (minutes > 0) {
                                                    return `${minutes}m ${seconds % 60}s`;
                                                } else {
                                                    return `${seconds}s`;
                                                }
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

export default WorkflowGanttChart;