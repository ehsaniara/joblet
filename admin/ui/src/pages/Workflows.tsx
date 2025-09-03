import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useWorkflows} from '../hooks/useWorkflows';
import WorkflowList from '../components/Workflow/WorkflowList';
import WorkflowDetail from '../components/Workflow/WorkflowDetail';
import {ArrowLeft, FileText, Folder, Plus, X} from 'lucide-react';
import {apiService} from '../services/apiService';

const Workflows: React.FC = () => {
    const {t} = useTranslation();
    const {
        paginatedWorkflows,
        loading,
        error,
        currentPage,
        pageSize,
        totalWorkflows,
        totalPages,
        setCurrentPage,
        setPageSize
    } = useWorkflows();

    // Initialize selected workflow from URL on component mount
    const getInitialWorkflowId = () => {
        const params = new URLSearchParams(window.location.search);
        return params.get('id');
    };

    const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(getInitialWorkflowId());
    const [createWorkflowModal, setCreateWorkflowModal] = useState({
        show: false,
        creating: false,
        error: ''
    });
    const [directoryBrowser, setDirectoryBrowser] = useState({
        currentPath: '',
        parentPath: null as string | null,
        directories: [] as Array<{ name: string; path: string; type: string }>,
        yamlFiles: [] as Array<{ name: string; path: string; type: string; selectable: boolean }>,
        otherFiles: [] as Array<{ name: string; path: string; type: string; selectable: boolean }>,
        loading: false,
        error: ''
    });
    const [selectedFile, setSelectedFile] = useState<string | null>(null);
    const [workflowValidation, setWorkflowValidation] = useState<{
        valid: boolean;
        requiredVolumes: string[];
        missingVolumes: string[];
        message: string;
        loading: boolean;
        error: string;
    }>({
        valid: true,
        requiredVolumes: [],
        missingVolumes: [],
        message: '',
        loading: false,
        error: ''
    });


    // Update URL when workflow selection changes
    useEffect(() => {
        const params = new URLSearchParams(window.location.search);
        if (selectedWorkflowId) {
            params.set('id', selectedWorkflowId);
        } else {
            params.delete('id');
        }
        const newUrl = params.toString() ? `${window.location.pathname}?${params.toString()}` : window.location.pathname;
        window.history.replaceState({}, '', newUrl);
    }, [selectedWorkflowId]);

    const handleWorkflowClick = (workflowId: string) => {
        setSelectedWorkflowId(workflowId);
    };


    // Handle escape key to close create workflow modal only
    useEffect(() => {
        const handleEscapeKey = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                if (createWorkflowModal.show && !createWorkflowModal.creating) {
                    // Close create workflow modal if open and not creating
                    resetWorkflowForm();
                }
                // Don't handle escape for workflow details - let WorkflowDetail component handle its own modals
            }
        };

        document.addEventListener('keydown', handleEscapeKey);
        return () => {
            document.removeEventListener('keydown', handleEscapeKey);
        };
    }, [createWorkflowModal.show, createWorkflowModal.creating]);

    const browseDirectory = async (path?: string) => {
        setDirectoryBrowser(prev => ({...prev, loading: true, error: ''}));

        try {
            const result = await apiService.browseWorkflowDirectory(path);
            setDirectoryBrowser({
                currentPath: result.currentPath,
                parentPath: result.parentPath,
                directories: result.directories,
                yamlFiles: result.yamlFiles,
                otherFiles: result.otherFiles,
                loading: false,
                error: ''
            });
        } catch (error) {
            setDirectoryBrowser(prev => ({
                ...prev,
                loading: false,
                error: error instanceof Error ? error.message : 'Failed to browse directory'
            }));
        }
    };

    const validateSelectedWorkflow = async (filePath: string) => {
        setWorkflowValidation(prev => ({...prev, loading: true, error: ''}));

        try {
            const validation = await apiService.validateWorkflow(filePath);
            setWorkflowValidation({
                valid: validation.valid,
                requiredVolumes: validation.requiredVolumes,
                missingVolumes: validation.missingVolumes,
                message: validation.message,
                loading: false,
                error: ''
            });
        } catch (error) {
            setWorkflowValidation(prev => ({
                ...prev,
                loading: false,
                error: error instanceof Error ? error.message : 'Failed to validate workflow'
            }));
        }
    };

    const handleFileSelection = async (filePath: string) => {
        setSelectedFile(filePath);
        await validateSelectedWorkflow(filePath);
    };

    const handleCreateWorkflow = async (createVolumes = false) => {
        if (!selectedFile) return;

        setCreateWorkflowModal(prev => ({...prev, creating: true}));

        try {
            await apiService.executeWorkflow(selectedFile, undefined, createVolumes);
            setCreateWorkflowModal({show: false, creating: false, error: ''});
            setSelectedFile(null);
            setWorkflowValidation({
                valid: true,
                requiredVolumes: [],
                missingVolumes: [],
                message: '',
                loading: false,
                error: ''
            });
            // Workflow list will auto-refresh via polling
        } catch (error) {
            console.error('Failed to create workflow:', error);
            const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred';
            setCreateWorkflowModal(prev => ({...prev, creating: false, error: errorMessage}));
        }
    };

    const resetWorkflowForm = () => {
        setSelectedFile(null);
        setDirectoryBrowser({
            currentPath: '',
            parentPath: null,
            directories: [],
            yamlFiles: [],
            otherFiles: [],
            loading: false,
            error: ''
        });
        setWorkflowValidation({
            valid: true,
            requiredVolumes: [],
            missingVolumes: [],
            message: '',
            loading: false,
            error: ''
        });
        setCreateWorkflowModal({show: false, creating: false, error: ''});
    };

    // Load directory browser when modal opens
    useEffect(() => {
        if (createWorkflowModal.show && !directoryBrowser.currentPath) {
            browseDirectory();
        }
    }, [createWorkflowModal.show]);

    // Show workflow detail view if a workflow ID is selected (even if not found in list)
    if (selectedWorkflowId) {
        return (
            <WorkflowDetail
                workflowId={selectedWorkflowId}
                onRefresh={() => {
                }} // No-op since auto-refresh is enabled
            />
        );
    }

    // Show workflow list view
    return (
        <div className="p-6">
            <div className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-white">Workflows</h1>
                        <p className="mt-2 text-gray-300">Visual workflow management and orchestration</p>
                        <div className="mt-2 flex items-center text-sm">
                            <div className="w-2 h-2 rounded-full mr-2 bg-green-500 animate-pulse"></div>
                            <span className="text-gray-400">Auto-refresh enabled (10s)</span>
                        </div>
                    </div>
                    <div>
                        <button
                            onClick={() => setCreateWorkflowModal({show: true, creating: false, error: ''})}
                            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
                        >
                            <Plus className="h-4 w-4 mr-2"/>
                            New Workflow
                        </button>
                    </div>
                </div>
            </div>

            {loading ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-white">{t('workflows.loadingWorkflows')}</p>
                    </div>
                </div>
            ) : error ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-red-500">{t('common.error')}: {error}</p>
                    </div>
                </div>
            ) : (
                <WorkflowList
                    workflows={paginatedWorkflows}
                    onWorkflowClick={handleWorkflowClick}
                    loading={loading}
                    currentPage={currentPage}
                    pageSize={pageSize}
                    totalWorkflows={totalWorkflows}
                    totalPages={totalPages}
                    setCurrentPage={setCurrentPage}
                    setPageSize={setPageSize}
                />
            )}

            {/* Create Workflow Modal */}
            {createWorkflowModal.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center p-4">
                    <div
                        className="relative bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] flex flex-col">
                        <div className="flex flex-col h-full min-h-0">
                            {/* Header */}
                            <div className="flex items-center justify-between p-6 border-b border-gray-600">
                                <h3 className="text-lg font-medium text-gray-200">Select Workflow File</h3>
                                <button
                                    onClick={resetWorkflowForm}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={createWorkflowModal.creating}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            {/* Content */}
                            <div className="flex-1 overflow-y-auto p-6 min-h-0">
                                {/* Current Path */}
                                <div className="mb-4">
                                    <div className="flex items-center space-x-2 text-sm text-gray-400">
                                        <span>Current Directory:</span>
                                        <span className="font-mono bg-gray-700 px-2 py-1 rounded">
                                            {directoryBrowser.currentPath || 'Loading...'}
                                        </span>
                                    </div>
                                </div>

                                {/* Error Message */}
                                {directoryBrowser.error && (
                                    <div
                                        className="mb-4 p-3 bg-red-800 bg-opacity-50 border border-red-600 rounded text-red-300">
                                        {directoryBrowser.error}
                                    </div>
                                )}

                                {/* Loading */}
                                {directoryBrowser.loading ? (
                                    <div className="flex items-center justify-center py-8">
                                        <div
                                            className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
                                        <span className="ml-3 text-gray-300">{t('workflows.loadingDirectory')}</span>
                                    </div>
                                ) : (
                                    <div className="space-y-4">
                                        {/* Navigation */}
                                        {directoryBrowser.parentPath && (
                                            <div>
                                                <button
                                                    onClick={() => browseDirectory(directoryBrowser.parentPath!)}
                                                    className="flex items-center space-x-2 px-3 py-2 text-gray-300 hover:bg-gray-700 rounded transition-colors"
                                                >
                                                    <ArrowLeft className="h-4 w-4"/>
                                                    <span>.. (Parent Directory)</span>
                                                </button>
                                            </div>
                                        )}

                                        {/* Two Column Layout: Directories and Files */}
                                        <div className="grid grid-cols-2 gap-6">
                                            {/* Left Column: Directories */}
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-300 mb-2">
                                                    Directories ({directoryBrowser.directories.length})
                                                </h4>
                                                <div className="border border-gray-600 rounded-lg">
                                                    {directoryBrowser.directories.length > 0 ? (
                                                        <div className="space-y-1 max-h-80 overflow-y-auto p-2">
                                                            {directoryBrowser.directories.map((dir) => (
                                                                <button
                                                                    key={dir.path}
                                                                    onClick={() => browseDirectory(dir.path)}
                                                                    className="flex items-center space-x-3 w-full px-3 py-2 text-left text-gray-300 hover:bg-gray-700 rounded transition-colors"
                                                                >
                                                                    <Folder className="h-4 w-4 text-blue-400"/>
                                                                    <span className="truncate">{dir.name}</span>
                                                                </button>
                                                            ))}
                                                        </div>
                                                    ) : (
                                                        <div className="text-center py-8 text-gray-500">
                                                            <Folder className="h-6 w-6 mx-auto mb-2 opacity-50"/>
                                                            <p className="text-sm">No directories</p>
                                                        </div>
                                                    )}
                                                </div>
                                            </div>

                                            {/* Right Column: Files */}
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-300 mb-2">
                                                    Files
                                                    ({directoryBrowser.yamlFiles.length + directoryBrowser.otherFiles.length})
                                                </h4>
                                                <div className="border border-gray-600 rounded-lg">
                                                    {(directoryBrowser.yamlFiles.length > 0 || directoryBrowser.otherFiles.length > 0) ? (
                                                        <div className="space-y-1 max-h-80 overflow-y-auto p-2">
                                                            {/* YAML Files (selectable) */}
                                                            {directoryBrowser.yamlFiles.map((file) => (
                                                                <button
                                                                    key={file.path}
                                                                    onClick={() => handleFileSelection(file.path)}
                                                                    className={`flex items-center space-x-3 w-full px-3 py-2 text-left rounded transition-colors ${
                                                                        selectedFile === file.path
                                                                            ? 'bg-blue-600 text-white'
                                                                            : 'text-gray-300 hover:bg-gray-700'
                                                                    }`}
                                                                >
                                                                    <FileText className="h-4 w-4 text-green-400"/>
                                                                    <span className="truncate flex-1">{file.name}</span>
                                                                    <span
                                                                        className="text-xs text-green-400 bg-green-900 px-2 py-1 rounded flex-shrink-0">
                                                                        YAML
                                                                    </span>
                                                                </button>
                                                            ))}

                                                            {/* Other Files (non-selectable) */}
                                                            {directoryBrowser.otherFiles.map((file) => (
                                                                <div
                                                                    key={file.path}
                                                                    className="flex items-center space-x-3 w-full px-3 py-2 text-gray-500 cursor-not-allowed opacity-60"
                                                                >
                                                                    <FileText className="h-4 w-4 text-gray-500"/>
                                                                    <span className="truncate flex-1">{file.name}</span>
                                                                    <span
                                                                        className="text-xs text-gray-500 bg-gray-700 px-2 py-1 rounded flex-shrink-0">
                                                                        Not selectable
                                                                    </span>
                                                                </div>
                                                            ))}
                                                        </div>
                                                    ) : !directoryBrowser.loading && (
                                                        <div className="text-center py-8 text-gray-500">
                                                            <FileText className="h-6 w-6 mx-auto mb-2 opacity-50"/>
                                                            <p className="text-sm">No files</p>
                                                            <p className="text-xs mt-1">
                                                                YAML files (.yaml/.yml) are selectable
                                                            </p>
                                                        </div>
                                                    )}
                                                </div>

                                                {/* Help text for YAML selection - moved outside the bordered area */}
                                                {directoryBrowser.yamlFiles.length === 0 && directoryBrowser.otherFiles.length > 0 && (
                                                    <div
                                                        className="mt-3 p-3 bg-yellow-800 bg-opacity-30 border border-yellow-600 rounded text-yellow-300 text-sm">
                                                        Only YAML files (.yaml/.yml) can be selected for workflow
                                                        execution.
                                                    </div>
                                                )}
                                            </div>
                                        </div>

                                        {/* Selected File Info */}
                                        {selectedFile && (
                                            <div className="mt-4 space-y-3">
                                                <div
                                                    className="p-3 bg-blue-800 bg-opacity-30 border border-blue-600 rounded">
                                                    <div className="flex items-center space-x-2">
                                                        <FileText className="h-4 w-4 text-blue-400"/>
                                                        <span className="text-blue-300 font-medium">Selected:</span>
                                                    </div>
                                                    <div className="mt-1 font-mono text-sm text-blue-200 break-all">
                                                        {selectedFile}
                                                    </div>
                                                </div>

                                                {/* Workflow Validation */}
                                                {workflowValidation.loading ? (
                                                    <div className="p-3 bg-gray-700 border border-gray-600 rounded">
                                                        <div className="flex items-center space-x-2">
                                                            <div
                                                                className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500"></div>
                                                            <span
                                                                className="text-gray-300">Validating workflow...</span>
                                                        </div>
                                                    </div>
                                                ) : workflowValidation.error ? (
                                                    <div
                                                        className="p-3 bg-red-800 bg-opacity-50 border border-red-600 rounded">
                                                        <div className="flex items-center space-x-2">
                                                            <span
                                                                className="text-red-300 font-medium">{t('workflows.validationError')}:</span>
                                                        </div>
                                                        <div className="mt-1 text-sm text-red-200">
                                                            {workflowValidation.error}
                                                        </div>
                                                    </div>
                                                ) : workflowValidation.missingVolumes.length > 0 ? (
                                                    <div
                                                        className="p-3 bg-yellow-800 bg-opacity-50 border border-yellow-600 rounded">
                                                        <div className="flex items-center space-x-2">
                                                            <span className="text-yellow-300 font-medium">Missing Dependencies:</span>
                                                        </div>
                                                        <div className="mt-2 text-sm text-yellow-200">
                                                            This workflow requires volumes that don't exist:
                                                        </div>
                                                        <ul className="mt-1 text-sm text-yellow-200 list-disc list-inside">
                                                            {workflowValidation.missingVolumes.map(volume => (
                                                                <li key={volume} className="font-mono">{volume}</li>
                                                            ))}
                                                        </ul>
                                                        <div className="mt-2 text-xs text-yellow-300">
                                                            You can create these volumes automatically when executing
                                                            the workflow.
                                                        </div>
                                                    </div>
                                                ) : workflowValidation.requiredVolumes.length > 0 ? (
                                                    <div
                                                        className="p-3 bg-green-800 bg-opacity-50 border border-green-600 rounded">
                                                        <div className="flex items-center space-x-2">
                                                            <span className="text-green-300 font-medium">Dependencies Satisfied:</span>
                                                        </div>
                                                        <div className="mt-1 text-sm text-green-200">
                                                            All required volumes are available:
                                                        </div>
                                                        <div className="mt-1 text-sm text-green-200 font-mono">
                                                            {workflowValidation.requiredVolumes.join(', ')}
                                                        </div>
                                                    </div>
                                                ) : null}
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>

                            {/* Error Display */}
                            {createWorkflowModal.error && (
                                <div className="px-6 py-4 bg-red-900/20 border-t border-gray-600">
                                    <div className="flex items-start space-x-3">
                                        <div
                                            className="w-6 h-6 rounded-full bg-red-500 flex-shrink-0 flex items-center justify-center mt-0.5">
                                            <X className="w-4 h-4 text-white"/>
                                        </div>
                                        <div className="flex-1">
                                            <h4 className="text-sm font-medium text-red-200 mb-2">Failed to Execute
                                                Workflow</h4>
                                            <div
                                                className="text-sm text-red-300 bg-red-900/30 p-3 rounded border border-red-700">
                                                <pre className="whitespace-pre-wrap font-mono text-xs leading-relaxed">
{createWorkflowModal.error}
                                                </pre>
                                            </div>
                                        </div>
                                        <button
                                            onClick={() => setCreateWorkflowModal(prev => ({...prev, error: ''}))}
                                            className="text-red-300 hover:text-red-100"
                                        >
                                            <X className="w-4 h-4"/>
                                        </button>
                                    </div>
                                </div>
                            )}

                            {/* Footer */}
                            <div className="flex space-x-3 justify-end p-6 border-t border-gray-600">
                                <button
                                    onClick={resetWorkflowForm}
                                    disabled={createWorkflowModal.creating}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                {workflowValidation.missingVolumes.length > 0 ? (
                                    <div className="flex space-x-2">
                                        <button
                                            onClick={() => handleCreateWorkflow(false)}
                                            disabled={createWorkflowModal.creating || !selectedFile}
                                            className="px-4 py-2 bg-gray-600 hover:bg-gray-700 disabled:bg-gray-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                        >
                                            {createWorkflowModal.creating ? (
                                                <>
                                                    <div
                                                        className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                                    Executing...
                                                </>
                                            ) : (
                                                <>
                                                    <Plus className="h-4 w-4 mr-2"/>
                                                    Execute Anyway
                                                </>
                                            )}
                                        </button>
                                        <button
                                            onClick={() => handleCreateWorkflow(true)}
                                            disabled={createWorkflowModal.creating || !selectedFile}
                                            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                        >
                                            {createWorkflowModal.creating ? (
                                                <>
                                                    <div
                                                        className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                                    Creating & Executing...
                                                </>
                                            ) : (
                                                <>
                                                    <Plus className="h-4 w-4 mr-2"/>
                                                    Create Volumes & Execute
                                                </>
                                            )}
                                        </button>
                                    </div>
                                ) : (
                                    <button
                                        onClick={() => handleCreateWorkflow(false)}
                                        disabled={createWorkflowModal.creating || !selectedFile}
                                        className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                    >
                                        {createWorkflowModal.creating ? (
                                            <>
                                                <div
                                                    className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                                Executing...
                                            </>
                                        ) : (
                                            <>
                                                <Plus className="h-4 w-4 mr-2"/>
                                                Execute Workflow
                                            </>
                                        )}
                                    </button>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Workflows;