import {GeneratedCommand, JobConfig, JobFlag} from '../types/job';

export class CommandBuilder {
    private command = '';
    private flags: Map<string, string | string[]> = new Map();

    // Constructor is intentionally empty - initialization done via field declarations

    static fromJobConfig(config: JobConfig): GeneratedCommand {
        const builder = new CommandBuilder()
            .setCommand(config.command)
            .setResource('cpu', config.maxCpu)
            .setResource('memory', config.maxMemory)
            .setResource('cores', config.cpuCores)
            .setResource('io', config.maxIobps)
            .setEnvironment(config.runtime, config.network)
            .setSchedule(config.schedule);

        // Add file uploads
        config.files.forEach(filePath => builder.addUpload(filePath));

        // Add directory uploads
        config.directories.forEach(dirPath => builder.addUploadDir(dirPath));

        // Add volumes
        config.volumes.forEach(volume => builder.addVolume(volume));

        // Add environment variables
        Object.entries(config.envVars).forEach(([key, value]) => {
            builder.addEnvVar(key, value);
        });

        return builder.build();
    }

    setCommand(cmd: string): this {
        this.command = cmd;
        return this;
    }

    addUpload(file: string): this {
        const uploads = this.flags.get('upload') as string[] || [];
        uploads.push(file);
        this.flags.set('upload', uploads);
        return this;
    }

    addUploadDir(dir: string): this {
        const uploadDirs = this.flags.get('upload-dir') as string[] || [];
        uploadDirs.push(dir);
        this.flags.set('upload-dir', uploadDirs);
        return this;
    }

    setResource(type: 'cpu' | 'memory' | 'io' | 'cores', value: number | string): this {
        const flagMap: Record<string, string> = {
            'cpu': 'max-cpu',
            'memory': 'max-memory',
            'io': 'max-iobps',
            'cores': 'cpu-cores'
        };

        if (value) {
            this.flags.set(flagMap[type], String(value));
        }
        return this;
    }

    setEnvironment(runtime: string, network: string): this {
        if (runtime) this.flags.set('runtime', runtime);
        if (network) this.flags.set('network', network);
        return this;
    }

    addVolume(volume: string): this {
        const volumes = this.flags.get('volume') as string[] || [];
        volumes.push(volume);
        this.flags.set('volume', volumes);
        return this;
    }

    addEnvVar(key: string, value: string): this {
        const envVars = this.flags.get('env') as string[] || [];
        envVars.push(`${key}=${value}`);
        this.flags.set('env', envVars);
        return this;
    }

    setSchedule(schedule: string): this {
        if (schedule) this.flags.set('schedule', schedule);
        return this;
    }


    build(): GeneratedCommand {
        const parts: string[] = ['rnx job run'];
        const flagsArray: JobFlag[] = [];

        // Build flag arguments with proper ordering
        const flagOrder = [
            'upload', 'upload-dir', 'max-cpu', 'max-memory', 'cpu-cores', 'max-iobps',
            'runtime', 'network', 'volume', 'env', 'schedule'
        ];

        flagOrder.forEach(flagName => {
            const value = this.flags.get(flagName);
            if (value !== undefined) {
                if (Array.isArray(value)) {
                    value.forEach(v => {
                        parts.push(`--${flagName}=${v}`);
                        flagsArray.push({flag: flagName, value: v, multiple: true});
                    });
                } else {
                    parts.push(`--${flagName}=${value}`);
                    flagsArray.push({flag: flagName, value});
                }
            }
        });

        parts.push(this.command);

        return {
            command: this.command,
            flags: flagsArray,
            fullCommand: parts.join(' ')
        };
    }
}