import express from 'express';
import fs from 'fs/promises';
import path from 'path';
import os from 'os';

const router = express.Router();

// Get user's .rnx directory path
const getRnxConfigPath = () => {
    const homeDir = os.homedir();
    return path.join(homeDir, '.rnx', 'admin-settings.json');
};

// Ensure .rnx directory exists
const ensureRnxDirectory = async () => {
    const homeDir = os.homedir();
    const rnxDir = path.join(homeDir, '.rnx');

    try {
        await fs.access(rnxDir);
    } catch (error) {
        if (error.code === 'ENOENT') {
            await fs.mkdir(rnxDir, {recursive: true});
        } else {
            throw error;
        }
    }
};

// Default settings
const defaultSettings = {
    refreshFrequency: 30,
    language: 'en',
    timezone: 'UTC'
};

// Get user settings
router.get('/', async (req, res) => {
    try {
        const settingsPath = getRnxConfigPath();

        try {
            const settingsData = await fs.readFile(settingsPath, 'utf8');
            const settings = JSON.parse(settingsData);
            res.json({...defaultSettings, ...settings});
        } catch (error) {
            if (error.code === 'ENOENT') {
                // Settings file doesn't exist, return defaults
                res.json(defaultSettings);
            } else {
                throw error;
            }
        }
    } catch (error) {
        console.error('Failed to load settings:', error);
        res.status(500).json({error: 'Failed to load settings'});
    }
});

// Save user settings
router.post('/', async (req, res) => {
    try {
        const {refreshFrequency, language, timezone} = req.body;

        // Validate settings
        const settings = {
            refreshFrequency: typeof refreshFrequency === 'number' ? refreshFrequency : defaultSettings.refreshFrequency,
            language: typeof language === 'string' ? language : defaultSettings.language,
            timezone: typeof timezone === 'string' ? timezone : defaultSettings.timezone
        };

        // Ensure .rnx directory exists
        await ensureRnxDirectory();

        // Save settings to .rnx/admin-settings.json
        const settingsPath = getRnxConfigPath();
        await fs.writeFile(settingsPath, JSON.stringify(settings, null, 2), 'utf8');

        res.json({
            success: true,
            message: 'Settings saved successfully',
            settings: settings,
            path: settingsPath
        });
    } catch (error) {
        console.error('Failed to save settings:', error);
        res.status(500).json({error: 'Failed to save settings'});
    }
});

export default router;