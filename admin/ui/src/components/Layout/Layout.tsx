import React from 'react';
import {Link, useLocation} from 'react-router-dom';
import {Activity, HardDrive, HelpCircle, Home, List, Settings, Workflow} from 'lucide-react';
import clsx from 'clsx';

interface LayoutProps {
    children: React.ReactNode;
}

const Layout: React.FC<LayoutProps> = ({children}) => {
    const location = useLocation();

    const navigation = [
        {name: 'Dashboard', href: '/', icon: Home},
        {name: 'Jobs', href: '/jobs', icon: List},
        {name: 'Workflows', href: '/workflows', icon: Workflow},
        {name: 'Monitoring', href: '/monitoring', icon: Activity},
        {name: 'Resources', href: '/resources', icon: HardDrive},
    ];

    return (
        <div className="flex h-screen bg-gray-100 dark:bg-gray-900">
            {/* Sidebar */}
            <div className="flex flex-col w-64 bg-white dark:bg-gray-800 shadow-lg">
                {/* Header */}
                <div className="flex items-center justify-between h-16 px-6 bg-blue-600 dark:bg-blue-700 text-white">
                    <h1 className="text-xl font-bold">Joblet Admin</h1>
                    <div className="flex space-x-2">
                        <button className="p-1 rounded hover:bg-blue-700 dark:hover:bg-blue-600">
                            <Settings size={18}/>
                        </button>
                        <button className="p-1 rounded hover:bg-blue-700 dark:hover:bg-blue-600">
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

                {/* Status */}
                <div className="px-4 py-4 border-t border-gray-200 dark:border-gray-700">
                    <div className="flex items-center text-sm text-gray-600 dark:text-gray-400">
                        <div className="w-2 h-2 bg-green-400 dark:bg-green-500 rounded-full mr-2"></div>
                        Connected to Joblet Server
                    </div>
                </div>
            </div>

            {/* Main content */}
            <div className="flex-1 flex flex-col overflow-hidden">
                <main className="flex-1 overflow-y-auto bg-gray-50 dark:bg-gray-900">
                    {children}
                </main>
            </div>
        </div>
    );
};

export default Layout;