// Quick test to verify Node.js is working in joblet
console.log('============================================');
console.log(' JOBLET NODE.JS TEST');
console.log('============================================');
console.log('Node.js version:', process.version);
console.log('Platform:', process.platform);
console.log('Architecture:', process.arch);
console.log('Working directory:', process.cwd());
console.log('Job ID:', process.env.JOB_ID || 'not set');
console.log('');
console.log('✅ Node.js is working correctly in joblet!');
console.log('============================================');