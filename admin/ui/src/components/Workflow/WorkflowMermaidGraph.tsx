import React, {useEffect, useRef} from 'react';
import mermaid from 'mermaid';
import {WorkflowJob} from '@/types';

interface WorkflowMermaidGraphProps {
    jobs: WorkflowJob[];
    onJobSelect?: (job: WorkflowJob | null) => void;
    onJobAction?: (jobId: string, action: string) => void;
}

const WorkflowMermaidGraph: React.FC<WorkflowMermaidGraphProps> = ({
                                                                       jobs,
                                                                       onJobSelect
                                                                   }) => {
    const mermaidRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        // Initialize Mermaid with dark theme
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
                mainBkg: '#374151',
                secondBkg: '#4b5563',
                tertiaryBkg: '#6b7280',
            },
            flowchart: {
                htmlLabels: true,
                curve: 'basis',
                useMaxWidth: true,
                rankSpacing: 50,
                nodeSpacing: 30,
            }
        });
    }, []);

    useEffect(() => {
        if (!mermaidRef.current || jobs.length === 0) return;

        // Generate Mermaid flowchart syntax
        const generateMermaidCode = () => {
            let code = 'flowchart TD\n';

            // Add nodes with status styling
            jobs.forEach(job => {
                const jobName = job.name || job.id;
                const sanitizedName = jobName.replace(/[^a-zA-Z0-9]/g, '_');
                const statusClass = getStatusClass(job.status);

                code += `    ${sanitizedName}["${jobName}"]:::${statusClass}\n`;
            });

            // Add edges based on dependencies
            jobs.forEach(job => {
                const jobName = job.name || job.id;
                const sanitizedName = jobName.replace(/[^a-zA-Z0-9]/g, '_');

                if (job.dependsOn && job.dependsOn.length > 0) {
                    job.dependsOn.forEach(dep => {
                        const depJob = jobs.find(j => j.name === dep || j.id === dep);
                        if (depJob) {
                            const depName = depJob.name || depJob.id;
                            const sanitizedDepName = depName.replace(/[^a-zA-Z0-9]/g, '_');
                            code += `    ${sanitizedDepName} --> ${sanitizedName}\n`;
                        }
                    });
                }
            });

            // Add CSS classes for different job statuses
            code += `
    classDef completed fill:#065f46,stroke:#10b981,stroke-width:2px,color:#f0fdf4
    classDef running fill:#92400e,stroke:#f59e0b,stroke-width:2px,color:#fffbeb
    classDef failed fill:#991b1b,stroke:#ef4444,stroke-width:2px,color:#fef2f2
    classDef pending fill:#1e40af,stroke:#3b82f6,stroke-width:2px,color:#eff6ff
    classDef stopped fill:#6b7280,stroke:#9ca3af,stroke-width:2px,color:#f9fafb
    classDef default fill:#374151,stroke:#6b7280,stroke-width:2px,color:#f9fafb
            `;

            return code;
        };

        const renderGraph = async () => {
            try {
                // Clean up any existing tooltips before re-rendering
                const existingTooltips = document.querySelectorAll('.graph-tooltip');
                existingTooltips.forEach(t => t.remove());

                const code = generateMermaidCode();
                console.log('Mermaid code:', code);

                // Clear previous content
                mermaidRef.current!.innerHTML = '';

                // Generate unique ID for this diagram
                const diagramId = `mermaid-${Date.now()}`;

                // Render the diagram
                const {svg} = await mermaid.render(diagramId, code);
                mermaidRef.current!.innerHTML = svg;

                // Add click handlers and hover tooltips to nodes
                const nodes = mermaidRef.current!.querySelectorAll('.node');
                nodes.forEach((node, index) => {
                    if (index < jobs.length) {
                        const job = jobs[index];
                        let tooltip: HTMLDivElement | null = null;

                        // Helper function to format time
                        const formatTime = (dateString: string) => {
                            return new Date(dateString).toLocaleTimeString('en-US', {
                                hour: '2-digit',
                                minute: '2-digit',
                                second: '2-digit',
                                hour12: false
                            });
                        };

                        // Helper function to format duration
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

                        // Add click handler
                        node.addEventListener('click', () => {
                            console.log('Job clicked:', job.name || job.id);
                            if (onJobSelect) {
                                onJobSelect(job);
                            }
                        });

                        // Add hover tooltip
                        node.addEventListener('mouseenter', (e) => {
                            node.setAttribute('style', 'cursor: pointer; opacity: 0.8;');

                            // Remove any existing tooltip first
                            const existingTooltips = document.querySelectorAll('.graph-tooltip');
                            existingTooltips.forEach(t => t.remove());

                            // Create tooltip
                            tooltip = document.createElement('div');
                            tooltip.className = 'graph-tooltip fixed bg-gray-700 text-white p-3 rounded shadow-lg text-xs z-50 pointer-events-none border border-gray-600';
                            tooltip.style.maxWidth = '250px';
                            tooltip.style.minWidth = '200px';

                            // Tooltip content with more information
                            let tooltipContent = `
                                <div class="font-semibold text-white">${job.name || job.id}</div>
                                <div class="text-xs text-gray-300 mt-1">UUID: ${job.id}</div>
                                <div class="mt-2 text-gray-200">Status: <span class="font-medium">${job.status}</span></div>
                            `;

                            if (job.command) {
                                tooltipContent += `<div class="text-gray-200">Command: <span class="font-mono text-gray-300">${job.command}</span></div>`;
                            }

                            if (job.startTime) {
                                tooltipContent += `<div class="text-gray-200">Start: <span class="font-mono text-gray-300">${formatTime(job.startTime)}</span></div>`;
                            }

                            if (job.endTime) {
                                tooltipContent += `<div class="text-gray-200">End: <span class="font-mono text-gray-300">${formatTime(job.endTime)}</span></div>`;
                            }

                            if (job.startTime && job.endTime) {
                                const duration = new Date(job.endTime).getTime() - new Date(job.startTime).getTime();
                                if (duration > 0) {
                                    tooltipContent += `<div class="text-gray-200">Duration: <span class="font-mono text-gray-300">${formatDuration(duration)}</span></div>`;
                                }
                            }

                            if (job.dependsOn && job.dependsOn.length > 0) {
                                tooltipContent += `
                                    <div class="mt-2 pt-2 border-t border-gray-600">
                                        <div class="text-gray-200">Depends on:</div>
                                        <div class="font-mono text-gray-300">${job.dependsOn.join(', ')}</div>
                                    </div>
                                `;
                            }

                            tooltip.innerHTML = tooltipContent;

                            // Position tooltip with smart positioning to stay within viewport
                            const rect = (e.target as Element).getBoundingClientRect();
                            const tooltipRect = {width: 250, height: 120}; // Estimated tooltip size

                            let left = rect.left + rect.width / 2;
                            let top = rect.top - 10;

                            // Adjust horizontal position to stay within viewport
                            if (left + tooltipRect.width / 2 > window.innerWidth - 20) {
                                left = window.innerWidth - tooltipRect.width - 20;
                                tooltip.style.transform = 'translateY(-100%)';
                            } else if (left - tooltipRect.width / 2 < 20) {
                                left = tooltipRect.width / 2 + 20;
                                tooltip.style.transform = 'translateY(-100%)';
                            } else {
                                tooltip.style.transform = 'translateX(-50%) translateY(-100%)';
                            }

                            // Adjust vertical position to stay within viewport
                            if (top - tooltipRect.height < 20) {
                                // Show below the node instead
                                top = rect.bottom + 10;
                                tooltip.style.transform = tooltip.style.transform.replace('translateY(-100%)', 'translateY(0%)');
                            }

                            tooltip.style.left = `${left}px`;
                            tooltip.style.top = `${top}px`;

                            // Add tooltip to body for better positioning control
                            document.body.appendChild(tooltip);
                        });

                        node.addEventListener('mouseleave', () => {
                            node.setAttribute('style', 'cursor: pointer; opacity: 1;');

                            // Remove tooltip with better cleanup
                            if (tooltip) {
                                if (tooltip.parentNode) {
                                    tooltip.parentNode.removeChild(tooltip);
                                }
                                tooltip = null;
                            }

                            // Also remove any orphaned tooltips
                            const orphanedTooltips = document.querySelectorAll('.graph-tooltip');
                            orphanedTooltips.forEach(t => {
                                if (t.parentNode) {
                                    t.parentNode.removeChild(t);
                                }
                            });
                        });

                        // Initial styling
                        node.setAttribute('style', 'cursor: pointer;');
                    }
                });

            } catch (error) {
                console.error('Error rendering Mermaid diagram:', error);
                mermaidRef.current!.innerHTML = `
                    <div class="flex items-center justify-center h-64 text-gray-400">
                        <div class="text-center">
                            <p>Error rendering workflow diagram</p>
                            <p class="text-sm mt-2">${error instanceof Error ? error.message : 'Unknown error'}</p>
                        </div>
                    </div>
                `;
            }
        };

        renderGraph();

        // Cleanup function to remove tooltips when component unmounts or jobs change
        return () => {
            const tooltips = document.querySelectorAll('.graph-tooltip');
            tooltips.forEach(t => t.remove());
        };
    }, [jobs, onJobSelect]);

    const getStatusClass = (status: string): string => {
        switch (status?.toUpperCase()) {
            case 'COMPLETED':
                return 'completed';
            case 'RUNNING':
                return 'running';
            case 'FAILED':
                return 'failed';
            case 'PENDING':
            case 'QUEUED':
                return 'pending';
            case 'STOPPED':
            case 'CANCELLED':
                return 'stopped';
            default:
                return 'default';
        }
    };

    if (jobs.length === 0) {
        return (
            <div className="p-4 h-full">
                <div className="bg-gray-800 rounded-lg shadow h-full flex flex-col">
                    <div className="p-4 border-b border-gray-700">
                        <div className="flex items-center justify-between">
                            <h3 className="text-lg font-medium text-white">Workflow Graph</h3>
                        </div>
                    </div>

                    <div className="flex-1 p-4 flex items-center justify-center">
                        <div className="text-center">
                            <p className="text-gray-400">No workflow jobs to display</p>
                            <p className="text-sm text-gray-500 mt-1">Jobs will appear here once the workflow is
                                executed</p>
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="p-4 h-full">
            <div className="bg-gray-800 rounded-lg shadow h-full flex flex-col">
                <div className="p-4 border-b border-gray-700">
                    <div className="flex items-center justify-between">
                        <h3 className="text-lg font-medium text-white">Workflow Graph</h3>
                    </div>
                </div>

                <div className="flex-1 p-4">
                    <div
                        ref={mermaidRef}
                        className="w-full h-full bg-gray-900 rounded-lg p-4 overflow-auto"
                        style={{minHeight: '500px', height: 'calc(100vh - 400px)'}}
                    />
                </div>
            </div>
        </div>
    );
};

export default WorkflowMermaidGraph;