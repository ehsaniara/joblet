#!/usr/bin/env node
/**
 * Simple Node.js Example - No External Dependencies
 * Works with Node.js built-in modules only
 */

const fs = require('fs');
const path = require('path');
const os = require('os');

function analyzeSystem() {
    console.log('📊 System Analysis with Node.js');
    console.log('================================');
    
    // System information
    console.log(`Platform: ${os.platform()}`);
    console.log(`Architecture: ${os.arch()}`);
    console.log(`Node.js Version: ${process.version}`);
    console.log(`CPU Count: ${os.cpus().length}`);
    console.log(`Total Memory: ${(os.totalmem() / 1024 / 1024 / 1024).toFixed(2)} GB`);
    console.log(`Free Memory: ${(os.freemem() / 1024 / 1024 / 1024).toFixed(2)} GB`);
    console.log('');
    
    // Process information
    console.log('🔧 Process Information');
    console.log('----------------------');
    console.log(`Process ID: ${process.pid}`);
    console.log(`Current Working Directory: ${process.cwd()}`);
    console.log(`Node.js Executable: ${process.execPath}`);
    console.log('');
    
    // Environment analysis
    console.log('🌍 Environment Variables');
    console.log('------------------------');
    const importantEnvVars = ['NODE_ENV', 'PATH', 'HOME', 'USER'];
    importantEnvVars.forEach(envVar => {
        if (process.env[envVar]) {
            console.log(`${envVar}: ${process.env[envVar].substring(0, 50)}${process.env[envVar].length > 50 ? '...' : ''}`);
        }
    });
    console.log('');
}

function processData() {
    console.log('📈 Data Processing Example');
    console.log('==========================');
    
    // Generate sample data
    const sampleData = [];
    for (let i = 1; i <= 100; i++) {
        sampleData.push({
            id: i,
            timestamp: new Date().toISOString(),
            value: Math.floor(Math.random() * 1000) + 1,
            category: ['A', 'B', 'C'][Math.floor(Math.random() * 3)]
        });
    }
    
    console.log(`Generated ${sampleData.length} data records`);
    
    // Analyze data
    const categoryStats = sampleData.reduce((acc, item) => {
        if (!acc[item.category]) {
            acc[item.category] = { count: 0, total: 0, values: [] };
        }
        acc[item.category].count++;
        acc[item.category].total += item.value;
        acc[item.category].values.push(item.value);
        return acc;
    }, {});
    
    console.log('\nCategory Analysis:');
    Object.entries(categoryStats).forEach(([category, stats]) => {
        const average = stats.total / stats.count;
        const min = Math.min(...stats.values);
        const max = Math.max(...stats.values);
        console.log(`  ${category}: Count=${stats.count}, Avg=${average.toFixed(2)}, Min=${min}, Max=${max}`);
    });
    
    // Save results if volume is available
    const volumePath = '/volumes/nodejs-data';
    if (fs.existsSync(volumePath)) {
        const resultsPath = path.join(volumePath, 'results.json');
        const results = {
            timestamp: new Date().toISOString(),
            totalRecords: sampleData.length,
            categories: categoryStats,
            summary: {
                totalValue: sampleData.reduce((sum, item) => sum + item.value, 0),
                averageValue: sampleData.reduce((sum, item) => sum + item.value, 0) / sampleData.length
            }
        };
        
        fs.writeFileSync(resultsPath, JSON.stringify(results, null, 2));
        console.log(`\n✅ Results saved to ${resultsPath}`);
    } else {
        console.log('\n📝 Volume not available - results not persisted');
    }
}

function demonstrateFileOperations() {
    console.log('📁 File Operations Example');
    console.log('==========================');
    
    // Create a temporary file
    const tempFile = '/tmp/nodejs_demo.txt';
    const content = `Node.js Demo File
Created at: ${new Date().toISOString()}
Process ID: ${process.pid}
Working Directory: ${process.cwd()}

This file demonstrates basic file operations in Node.js.
`;
    
    fs.writeFileSync(tempFile, content);
    console.log(`✅ Created file: ${tempFile}`);
    
    // Read it back
    const readContent = fs.readFileSync(tempFile, 'utf8');
    console.log('📖 File contents:');
    console.log(readContent);
    
    // File stats
    const stats = fs.statSync(tempFile);
    console.log(`📊 File size: ${stats.size} bytes`);
    console.log(`📅 Created: ${stats.birthtime.toISOString()}`);
    
    // Clean up
    fs.unlinkSync(tempFile);
    console.log('🧹 Temporary file cleaned up');
}

// Main execution
function main() {
    console.log('🚀 Node.js Joblet Demo');
    console.log('======================');
    console.log('This demo uses only Node.js built-in modules');
    console.log('');
    
    analyzeSystem();
    processData();
    demonstrateFileOperations();
    
    console.log('');
    console.log('✅ Node.js demo completed successfully!');
}

// Run the demo
main();