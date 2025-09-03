import {exec} from 'child_process';
import {promisify} from 'util';
import {config} from '../config.js';

const execAsync = promisify(exec);

/**
 * Execute rnx commands with proper node selection and error handling
 */
export async function execRnx(args, options = {}) {
    try {
        // Add node selection if provided
        const node = options.node;
        if (node && node !== 'default') {
            args = ['--node', node, ...args];
        }

        const command = `${config.RNX_PATH} ${args.join(' ')}`;
        const {stdout, stderr} = await execAsync(command, options);

        if (stderr) {
            console.warn(`Command warning: ${stderr}`);
        }

        return stdout.trim();
    } catch (error) {
        console.error(`Command failed: ${error.message}`);
        throw error;
    }
}