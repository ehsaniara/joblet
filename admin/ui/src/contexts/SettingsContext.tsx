import React, {createContext, useContext, useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';

interface UserSettings {
    refreshFrequency: number; // seconds
    language: string;
    timezone: string;
}

interface SettingsContextType {
    settings: UserSettings;
    updateSettings: (newSettings: Partial<UserSettings>) => void;
    refreshSettings: () => Promise<void>;
}

const defaultSettings: UserSettings = {
    refreshFrequency: 30,
    language: 'en',
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
};

const SettingsContext = createContext<SettingsContextType | undefined>(undefined);

export const SettingsProvider: React.FC<{ children: React.ReactNode }> = ({children}) => {
    const [settings, setSettings] = useState<UserSettings>(defaultSettings);
    const {i18n} = useTranslation();

    const refreshSettings = async () => {
        try {
            const response = await fetch('/api/settings');
            if (response.ok) {
                const savedSettings = await response.json();
                const newSettings = {...defaultSettings, ...savedSettings};
                setSettings(newSettings);
                // Update i18n language when settings are loaded
                if (newSettings.language !== i18n.language) {
                    i18n.changeLanguage(newSettings.language);
                }
            }
        } catch (error) {
            console.warn('Failed to load settings:', error);
        }
    };

    const updateSettings = (newSettings: Partial<UserSettings>) => {
        const updatedSettings = {...settings, ...newSettings};
        setSettings(updatedSettings);

        // Update i18n language when language setting changes
        if (newSettings.language && newSettings.language !== i18n.language) {
            i18n.changeLanguage(newSettings.language);
        }
    };

    useEffect(() => {
        refreshSettings();
    }, []);

    return (
        <SettingsContext.Provider value={{settings, updateSettings, refreshSettings}}>
            {children}
        </SettingsContext.Provider>
    );
};

export const useSettings = () => {
    const context = useContext(SettingsContext);
    if (context === undefined) {
        throw new Error('useSettings must be used within a SettingsProvider');
    }
    return context;
};

export type {UserSettings};