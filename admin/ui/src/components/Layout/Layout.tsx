// React import not needed with modern JSX transform
import {Link, useLocation} from 'react-router-dom';
import {Activity, HardDrive, HelpCircle, Home, List, Save, Settings, Workflow, X} from 'lucide-react';
import clsx from 'clsx';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import NodeSelector from '../NodeSelector/NodeSelector';
import {useNode} from '../../contexts/NodeContext';
import {UserSettings, useSettings} from '../../contexts/SettingsContext';

interface LayoutProps {
    children: React.ReactNode;
}

const Layout: React.FC<LayoutProps> = ({children}) => {
    const location = useLocation();
    const {selectedNode, setSelectedNode} = useNode();
    const {settings, updateSettings} = useSettings();
    const {t} = useTranslation();
    const [showSettings, setShowSettings] = useState(false);
    const [settingsForm, setSettingsForm] = useState<UserSettings>(settings);
    const [savingSettings, setSavingSettings] = useState(false);
    const [showHelp, setShowHelp] = useState(false);
    const [rnxVersion, setRnxVersion] = useState<string>('');

    const navigation = [
        {name: t('nav.dashboard'), href: '/', icon: Home},
        {name: t('nav.jobs'), href: '/jobs', icon: List},
        {name: t('nav.workflows'), href: '/workflows', icon: Workflow},
        {name: t('nav.monitoring'), href: '/monitoring', icon: Activity},
        {name: t('nav.resources'), href: '/resources', icon: HardDrive},
    ];

    const saveSettings = async () => {
        setSavingSettings(true);
        try {
            const response = await fetch('/api/settings', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(settingsForm),
            });

            if (response.ok) {
                updateSettings(settingsForm);
                setShowSettings(false);
            } else {
                console.error('Failed to save settings');
            }
        } catch (error) {
            console.error('Failed to save settings:', error);
        } finally {
            setSavingSettings(false);
        }
    };

    const openSettings = () => {
        setSettingsForm(settings);
        setShowSettings(true);
    };

    const closeSettings = () => {
        setSettingsForm(settings);
        setShowSettings(false);
    };

    const fetchVersion = async () => {
        try {
            const response = await fetch(`/api/version?node=${selectedNode}`);
            if (response.ok) {
                const versionData = await response.json();
                if (versionData.rnx) {
                    setRnxVersion(versionData.rnx);
                }
            }
        } catch (error) {
            console.error('Failed to fetch version:', error);
        }
    };

    // Fetch version information
    useEffect(() => {
        fetchVersion();
    }, [selectedNode]);

    // Handle escape key to close dialogs
    useEffect(() => {
        const handleEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                if (showSettings) {
                    closeSettings();
                } else if (showHelp) {
                    setShowHelp(false);
                }
            }
        };

        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [showSettings, showHelp]);

    return (
        <div className="flex h-screen bg-gray-100 dark:bg-gray-900">
            {/* Sidebar */}
            <div className="flex flex-col w-64 bg-white dark:bg-gray-800 shadow-lg">
                {/* Header */}
                <div className="flex items-center justify-between h-16 px-6 bg-blue-600 dark:bg-blue-700 text-white">
                    <h1 className="text-xl font-bold">Joblet Admin</h1>
                    <div className="flex space-x-2">
                        <button
                            onClick={openSettings}
                            className="p-1 rounded hover:bg-blue-700 dark:hover:bg-blue-600"
                            title={t('common.settings')}
                        >
                            <Settings size={18}/>
                        </button>
                        <button
                            onClick={() => setShowHelp(true)}
                            className="p-1 rounded hover:bg-blue-700 dark:hover:bg-blue-600"
                            title={t('common.help')}
                        >
                            <HelpCircle size={18}/>
                        </button>
                    </div>
                </div>

                {/* Navigation */}
                <nav className="flex-1 px-4 py-6 space-y-2">
                    {navigation.map((item) => {
                        const Icon = item.icon;
                        const isActive = location.pathname === item.href;

                        return (
                            <Link
                                key={item.name}
                                to={item.href}
                                className={clsx(
                                    'flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-colors',
                                    isActive
                                        ? 'bg-blue-50 dark:bg-blue-900 text-blue-700 dark:text-blue-300 border-r-2 border-blue-700 dark:border-blue-400'
                                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white'
                                )}
                            >
                                <Icon
                                    size={18}
                                    className={clsx(
                                        'mr-3',
                                        isActive ? 'text-blue-700 dark:text-blue-300' : 'text-gray-400 dark:text-gray-500'
                                    )}
                                />
                                {item.name}
                            </Link>
                        );
                    })}
                </nav>

                {/* Node Selector */}
                <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-700">
                    <NodeSelector
                        selectedNode={selectedNode}
                        onNodeChange={setSelectedNode}
                    />
                </div>

                {/* Status */}
                <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-700">
                    <div className="flex items-center text-sm text-gray-600 dark:text-gray-400">
                        <div className="w-2 h-2 bg-green-400 dark:bg-green-500 rounded-full mr-2"></div>
                        Connected: {selectedNode}
                    </div>
                    {rnxVersion && (
                        <div className="mt-1 text-xs text-gray-500 dark:text-gray-500 ml-4">
                            rnx {rnxVersion}
                        </div>
                    )}
                </div>
            </div>

            {/* Main content */}
            <div className="flex-1 flex flex-col overflow-hidden">
                <main className="flex-1 overflow-y-auto bg-gray-50 dark:bg-gray-900">
                    {children}
                </main>
            </div>

            {/* Help Dialog */}
            {showHelp && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center p-4">
                    <div
                        className="relative bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] flex flex-col">
                        <div className="flex-shrink-0 p-6 border-b border-gray-600">
                            <div className="flex items-center justify-between">
                                <h3 className="text-xl font-medium text-gray-200">{t('help.title')}</h3>
                                <button
                                    onClick={() => setShowHelp(false)}
                                    className="text-gray-400 hover:text-gray-300"
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>
                        </div>
                        <div className="flex-1 overflow-y-auto p-6">
                            <div className="space-y-6 text-gray-300">
                                {/* Overview */}
                                <div>
                                    <h4 className="text-lg font-semibold text-gray-200 mb-3">Overview</h4>
                                    <p className="mb-2">
                                        Joblet provides an enterprise-grade job execution platform with a modern
                                        React-based web interface for visual workflow management.
                                    </p>
                                </div>

                                {/* Main Features */}
                                <div>
                                    <h4 className="text-lg font-semibold text-gray-200 mb-3">Main Sections</h4>
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <div className="flex items-center">
                                                <Home className="h-4 w-4 mr-2 text-blue-400"/>
                                                <strong>Dashboard:</strong> System health overview
                                            </div>
                                            <div className="flex items-center">
                                                <List className="h-4 w-4 mr-2 text-green-400"/>
                                                <strong>Jobs:</strong> Job list, details, and creation
                                            </div>
                                            <div className="flex items-center">
                                                <Workflow className="h-4 w-4 mr-2 text-purple-400"/>
                                                <strong>Workflows:</strong> YAML-based workflow management
                                            </div>
                                        </div>
                                        <div className="space-y-2">
                                            <div className="flex items-center">
                                                <Activity className="h-4 w-4 mr-2 text-yellow-400"/>
                                                <strong>Monitoring:</strong> Real-time system metrics
                                            </div>
                                            <div className="flex items-center">
                                                <HardDrive className="h-4 w-4 mr-2 text-red-400"/>
                                                <strong>Resources:</strong> Volume and network management
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                {/* Key Features */}
                                <div>
                                    <h4 className="text-lg font-semibold text-gray-200 mb-3">Key Features</h4>
                                    <ul className="list-disc list-inside space-y-1">
                                        <li>Real-time metrics with comprehensive system observability</li>
                                        <li>Workflow orchestration with job dependency validation</li>
                                        <li>Live log streaming with advanced filtering</li>
                                        <li>Visual dependency mapping and timeline views</li>
                                        <li>Role-based access control via mTLS authentication</li>
                                        <li>Auto-refresh functionality (configurable in Settings)</li>
                                    </ul>
                                </div>

                                {/* Getting Started */}
                                <div>
                                    <h4 className="text-lg font-semibold text-gray-200 mb-3">Getting Started</h4>
                                    <ol className="list-decimal list-inside space-y-1">
                                        <li>Create jobs using the "New Job" button in the Jobs section</li>
                                        <li>Configure workflows with YAML files</li>
                                        <li>Monitor job executions in real-time</li>
                                        <li>Use the Dashboard for system health overview</li>
                                        <li>Adjust settings using the Settings icon in the header</li>
                                    </ol>
                                </div>

                                {/* Quick Tips */}
                                <div>
                                    <h4 className="text-lg font-semibold text-gray-200 mb-3">Quick Tips</h4>
                                    <ul className="list-disc list-inside space-y-1">
                                        <li>Use built-in runtime environments for consistency</li>
                                        <li>Set resource limits for jobs to prevent system overload</li>
                                        <li>Leverage network isolation features for security</li>
                                        <li>Monitor job dependencies and status regularly</li>
                                        <li>Configure auto-refresh frequency based on your needs</li>
                                    </ul>
                                </div>

                                {/* Authentication */}
                                <div>
                                    <h4 className="text-lg font-semibold text-gray-200 mb-3">Authentication</h4>
                                    <p>
                                        Authentication uses mTLS certificates. No separate login is required when
                                        properly configured.
                                    </p>
                                </div>

                                {/* Support */}
                                <div>
                                    <h4 className="text-lg font-semibold text-gray-200 mb-3">Need More Help?</h4>
                                    <p>
                                        Visit the <a href="https://github.com/ehsaniara/joblet/tree/main/docs"
                                                     target="_blank" rel="noopener noreferrer"
                                                     className="text-blue-400 hover:text-blue-300 underline">
                                        full documentation on GitHub
                                    </a> for detailed guides on configuration, security, monitoring, and more.
                                    </p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Settings Dialog */}
            {showSettings && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-6">
                                <h3 className="text-lg font-medium text-gray-200">{t('settings.title')}</h3>
                                <button
                                    onClick={closeSettings}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={savingSettings}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-6">
                                {/* Auto-refresh Frequency */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        {t('settings.autoRefreshFrequency')}
                                    </label>
                                    <select
                                        value={settingsForm.refreshFrequency}
                                        onChange={(e) => setSettingsForm(prev => ({
                                            ...prev,
                                            refreshFrequency: parseInt(e.target.value)
                                        }))}
                                        className="w-full px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        disabled={savingSettings}
                                    >
                                        <option value={5}>{t('settings.frequencies.5sec')}</option>
                                        <option value={10}>{t('settings.frequencies.10sec')}</option>
                                        <option value={30}>{t('settings.frequencies.30sec')}</option>
                                        <option value={60}>{t('settings.frequencies.1min')}</option>
                                        <option value={300}>{t('settings.frequencies.5min')}</option>
                                        <option value={0}>{t('settings.frequencies.disabled')}</option>
                                    </select>
                                    <p className="text-xs text-gray-400 mt-1">
                                        {t('settings.autoRefreshHelp')}
                                    </p>
                                </div>

                                {/* Language */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        {t('settings.language')}
                                    </label>
                                    <select
                                        value={settingsForm.language}
                                        onChange={(e) => setSettingsForm(prev => ({...prev, language: e.target.value}))}
                                        className="w-full px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        disabled={savingSettings}
                                    >
                                        <option value="en">English</option>
                                        <option value="es">Español</option>
                                        <option value="fr">Français</option>
                                        <option value="de">Deutsch</option>
                                        <option value="ja">日本語</option>
                                        <option value="zh">中文</option>
                                    </select>
                                </div>

                                {/* Timezone */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        {t('settings.timezone')}
                                    </label>
                                    <select
                                        value={settingsForm.timezone}
                                        onChange={(e) => setSettingsForm(prev => ({...prev, timezone: e.target.value}))}
                                        className="w-full px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        disabled={savingSettings}
                                    >
                                        <option value="UTC">UTC</option>
                                        <option value="America/New_York">Eastern Time</option>
                                        <option value="America/Chicago">Central Time</option>
                                        <option value="America/Denver">Mountain Time</option>
                                        <option value="America/Los_Angeles">Pacific Time</option>
                                        <option value="Europe/London">London</option>
                                        <option value="Europe/Paris">Paris</option>
                                        <option value="Europe/Berlin">Berlin</option>
                                        <option value="Asia/Tokyo">Tokyo</option>
                                        <option value="Asia/Shanghai">Shanghai</option>
                                        <option value="Asia/Kolkata">Mumbai</option>
                                        <option value="Australia/Sydney">Sydney</option>
                                        <option value={Intl.DateTimeFormat().resolvedOptions().timeZone}>
                                            System Default ({Intl.DateTimeFormat().resolvedOptions().timeZone})
                                        </option>
                                    </select>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-8">
                                <button
                                    onClick={closeSettings}
                                    disabled={savingSettings}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    {t('common.cancel')}
                                </button>
                                <button
                                    onClick={saveSettings}
                                    disabled={savingSettings}
                                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {savingSettings ? (
                                        <>
                                            <div
                                                className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            {t('settings.saving')}
                                        </>
                                    ) : (
                                        <>
                                            <Save className="h-4 w-4 mr-2"/>
                                            {t('settings.saveSettings')}
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Layout;