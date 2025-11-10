import { App, Plugin, PluginSettingTab, Setting, TFile, Notice } from 'obsidian';
import { exec } from 'child_process';
import { normalize } from 'path';

// Define the structure for the plugin settings
interface GoAppRunnerSettings {
    goExecutableName: string; // e.g., 'my-go-tool.exe' or 'my-go-tool'
}

const DEFAULT_SETTINGS: GoAppRunnerSettings = {
    goExecutableName: 'my-go-tool',
}

export default class GoAppRunnerPlugin extends Plugin {
    settings: GoAppRunnerSettings;

    async onload() {
        // Load saved settings
        await this.loadSettings();

        console.log('GoApp Runner Plugin Loaded');

        // 1. Add a ribbon icon that executes the Go app
        this.addRibbonIcon('terminal', 'Run Go Tool', async () => {
            await this.runGoAppCommand('Ribbon Click');
        });

        // 2. Add a command to the Command Palette
        this.addCommand({
            id: 'run-go-tool',
            name: 'Run Go Tool with Active Note Path',
            callback: async () => {
                const activeFile = this.app.workspace.getActiveFile();
                if (activeFile instanceof TFile) {
                    // Pass the full path to the active file as an argument
                    const path = this.app.vault.adapter.getFullPath(activeFile.path);
                    await this.runGoAppCommand(path);
                } else {
                    new Notice('No active note open to pass to the Go Tool.', 3000);
                }
            }
        });

        // 3. Add a settings tab
        this.addSettingTab(new GoAppRunnerSettingTab(this.app, this));
    }

    onunload() {
        console.log('GoApp Runner Plugin Unloaded');
    }

    async loadSettings() {
        this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
    }

    async saveSettings() {
        await this.saveData(this.settings);
    }

    /**
     * Executes the external Go application using Node.js's child_process.
     * @param argument The argument to pass to the external executable.
     */
    async runGoAppCommand(argument: string) {
        const pluginPath = this.manifest.dir;
        const executableName = this.settings.goExecutableName;

        // The executable is assumed to be in the root of the plugin directory
        const executablePath = normalize(`${pluginPath}/${executableName}`);

        // Construct the command. Ensure the argument is correctly escaped/quoted.
        const command = `${executablePath} "${argument}"`;

        new Notice(`Executing Go Tool: ${command}`, 4000);

        // Execute the command asynchronously
        exec(command, (error, stdout, stderr) => {
            if (error) {
                console.error(`Go App Execution Error: ${error.message}`);
                new Notice(`Go Tool Error: ${error.message}`, 10000);
                return;
            }
            if (stderr) {
                console.error(`Go App STDERR: ${stderr}`);
                new Notice(`Go Tool Stderr: ${stderr}`, 10000);
                // Still show stdout if there is stderr
            }

            // Show success notification with the output
            const output = stdout.trim();
            if (output) {
                new Notice(`Go Tool Success! Output:\n${output}`, 10000);
                console.log(`Go App STDOUT: ${output}`);
            } else {
                new Notice('Go Tool executed, but returned no output.', 4000);
            }
        });
    }
}

// --- SETTINGS TAB ---

class GoAppRunnerSettingTab extends PluginSettingTab {
    plugin: GoAppRunnerPlugin;

    constructor(app: App, plugin: GoAppRunnerPlugin) {
        super(app, plugin);
        this.plugin = plugin;
    }

    display(): void {
        const { containerEl } = this;

        containerEl.empty();
        containerEl.createEl('h2', { text: 'Go App Runner Settings' });

        new Setting(containerEl)
            .setName('Go Executable Filename')
            .setDesc('The filename of your compiled Go application (e.g., my-go-tool or my-go-tool.exe). This file must be placed inside the plugin folder.')
            .addText(text => text
                .setPlaceholder('my-go-tool')
                .setValue(this.plugin.settings.goExecutableName)
                .onChange(async (value) => {
                    this.plugin.settings.goExecutableName = value;
                    await this.plugin.saveSettings();
                }));
    }
}