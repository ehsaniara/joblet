import React from 'react';
import {Cpu, HardDrive, Network} from 'lucide-react';

const Resources: React.FC = () => {
    return (
        <div className="p-6">
            <div className="mb-8">
                <h1 className="text-3xl font-bold text-gray-900">Resources</h1>
                <p className="mt-2 text-gray-600">Manage volumes, networks, and runtime environments</p>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Volumes */}
                <div className="bg-white rounded-lg shadow p-6">
                    <div className="flex items-center mb-4">
                        <HardDrive className="h-6 w-6 text-blue-600 mr-3"/>
                        <h3 className="text-lg font-semibold text-gray-900">Volumes</h3>
                    </div>
                    <div className="text-center py-8">
                        <p className="text-gray-500 mb-4">No volumes configured</p>
                        <button
                            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700">
                            Create Volume
                        </button>
                    </div>
                </div>

                {/* Networks */}
                <div className="bg-white rounded-lg shadow p-6">
                    <div className="flex items-center mb-4">
                        <Network className="h-6 w-6 text-green-600 mr-3"/>
                        <h3 className="text-lg font-semibold text-gray-900">Networks</h3>
                    </div>
                    <div className="text-center py-8">
                        <p className="text-gray-500 mb-4">Default bridge network active</p>
                        <button
                            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-green-600 hover:bg-green-700">
                            Manage Networks
                        </button>
                    </div>
                </div>

                {/* Runtimes */}
                <div className="bg-white rounded-lg shadow p-6">
                    <div className="flex items-center mb-4">
                        <Cpu className="h-6 w-6 text-purple-600 mr-3"/>
                        <h3 className="text-lg font-semibold text-gray-900">Runtimes</h3>
                    </div>
                    <div className="text-center py-8">
                        <p className="text-gray-500 mb-4">Runtime environments available</p>
                        <button
                            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-purple-600 hover:bg-purple-700">
                            View Runtimes
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Resources;