import React from 'react';
import {useNavigate} from 'react-router-dom';
import {SimpleJobBuilder} from '../components/JobBuilder/SimpleJobBuilder';
import {ArrowLeft} from 'lucide-react';

const CreateJob: React.FC = () => {
    const navigate = useNavigate();

    const handleJobCreated = (jobId: string) => {
        // Show success message and redirect to jobs page
        console.log('Job created:', jobId);
        navigate('/jobs', {
            state: {
                message: `Job ${jobId} created successfully`,
                type: 'success'
            }
        });
    };

    const handleClose = () => {
        navigate('/jobs');
    };

    return (
        <div className="min-h-full bg-gray-50">
            {/* Header */}
            <div className="bg-white shadow">
                <div className="px-6 py-4">
                    <div className="flex items-center">
                        <button
                            onClick={handleClose}
                            className="flex items-center text-gray-600 hover:text-gray-900 mr-4"
                        >
                            <ArrowLeft className="w-5 h-5 mr-1"/>
                            Back to Jobs
                        </button>
                        <h1 className="text-2xl font-bold text-gray-900">Create New Job</h1>
                    </div>
                </div>
            </div>

            {/* Content */}
            <div className="py-8">
                <SimpleJobBuilder
                    onJobCreated={handleJobCreated}
                    onClose={handleClose}
                />
            </div>
        </div>
    );
};

export default CreateJob;