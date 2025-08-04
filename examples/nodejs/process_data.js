const fs = require('fs');
const path = require('path');
const csv = require('csv-parser');
const { Transform } = require('stream');
const createCsvWriter = require('csv-writer').createObjectCsvWriter;

class DataProcessor {
  constructor(config) {
    this.config = config || {};
    this.inputDir = '/volumes/nodejs-projects/input';
    this.outputDir = '/volumes/nodejs-projects/output';
    this.statsDir = '/volumes/nodejs-projects/stats';
    
    // Create directories
    [this.outputDir, this.statsDir].forEach(dir => {
      if (fs.existsSync('/volumes/nodejs-projects')) {
        fs.mkdirSync(dir, { recursive: true });
      }
    });
    
    this.stats = {
      totalRecords: 0,
      processedRecords: 0,
      errorRecords: 0,
      startTime: new Date(),
      endTime: null
    };
  }

  // Transform stream for data processing
  createTransformStream() {
    return new Transform({
      objectMode: true,
      transform(chunk, encoding, callback) {
        try {
          // Data transformation logic
          const transformed = {
            id: chunk.id,
            name: chunk.name?.trim().toLowerCase(),
            email: chunk.email?.toLowerCase(),
            age: parseInt(chunk.age) || null,
            salary: parseFloat(chunk.salary) || null,
            department: chunk.department?.trim(),
            joinDate: new Date(chunk.join_date).toISOString().split('T')[0],
            isActive: chunk.status === 'active',
            // Add calculated fields
            salaryGrade: this.calculateSalaryGrade(parseFloat(chunk.salary)),
            experienceYears: this.calculateExperience(chunk.join_date),
            processedAt: new Date().toISOString()
          };

          this.stats.processedRecords++;
          callback(null, transformed);
          
        } catch (error) {
          this.stats.errorRecords++;
          console.error(`Error processing record ${chunk.id}:`, error.message);
          callback(); // Skip invalid records
        }
      }.bind(this)
    });
  }

  calculateSalaryGrade(salary) {
    if (!salary) return 'unknown';
    if (salary < 30000) return 'entry';
    if (salary < 60000) return 'junior';
    if (salary < 100000) return 'senior';
    return 'executive';
  }

  calculateExperience(joinDate) {
    const join = new Date(joinDate);
    const now = new Date();
    return Math.floor((now - join) / (365.25 * 24 * 60 * 60 * 1000));
  }

  async processFile(inputFile, outputFile) {
    return new Promise((resolve, reject) => {
      console.log(`Processing ${inputFile}...`);
      
      const csvWriter = createCsvWriter({
        path: outputFile,
        header: [
          {id: 'id', title: 'ID'},
          {id: 'name', title: 'Name'},
          {id: 'email', title: 'Email'},
          {id: 'age', title: 'Age'},
          {id: 'salary', title: 'Salary'},
          {id: 'department', title: 'Department'},
          {id: 'joinDate', title: 'Join Date'},
          {id: 'isActive', title: 'Active'},
          {id: 'salaryGrade', title: 'Salary Grade'},
          {id: 'experienceYears', title: 'Experience Years'},
          {id: 'processedAt', title: 'Processed At'}
        ]
      });

      const transformedRecords = [];
      
      fs.createReadStream(inputFile)
        .pipe(csv())
        .on('data', (data) => {
          this.stats.totalRecords++;
        })
        .pipe(this.createTransformStream())
        .on('data', (transformed) => {
          transformedRecords.push(transformed);
        })
        .on('end', async () => {
          try {
            await csvWriter.writeRecords(transformedRecords);
            console.log(`Processed ${transformedRecords.length} records to ${outputFile}`);
            resolve(transformedRecords.length);
          } catch (error) {
            reject(error);
          }
        })
        .on('error', reject);
    });
  }

  async generateReport(processedFiles) {
    this.stats.endTime = new Date();
    this.stats.processingTimeMs = this.stats.endTime - this.stats.startTime;
    
    const report = {
      summary: this.stats,
      filesProcessed: processedFiles,
      performance: {
        recordsPerSecond: Math.round(this.stats.processedRecords / (this.stats.processingTimeMs / 1000)),
        memoryUsage: process.memoryUsage()
      }
    };

    // Save report if volume exists
    if (fs.existsSync('/volumes/nodejs-projects')) {
      const reportPath = path.join(this.statsDir, `processing-report-${Date.now()}.json`);
      fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
      console.log(`Report saved: ${reportPath}`);
    }
    
    console.log('\nProcessing Report:');
    console.log(`Total Records: ${this.stats.totalRecords}`);
    console.log(`Processed Records: ${this.stats.processedRecords}`);
    console.log(`Error Records: ${this.stats.errorRecords}`);
    console.log(`Processing Time: ${this.stats.processingTimeMs}ms`);
    console.log(`Records/Second: ${report.performance.recordsPerSecond}`);
    
    return report;
  }

  async run() {
    try {
      console.log('Starting data processing...');
      
      // Create sample data if no input directory exists
      if (!fs.existsSync(this.inputDir)) {
        console.log('Creating sample employee data...');
        this.createSampleData();
      }
      
      // Get all CSV files in input directory
      const files = fs.readdirSync(this.inputDir)
        .filter(file => file.endsWith('.csv'));
      
      if (files.length === 0) {
        console.log('No CSV files found in input directory');
        return;
      }

      const processedFiles = [];
      
      // Process each file
      for (const file of files) {
        const inputPath = path.join(this.inputDir, file);
        const outputPath = path.join(this.outputDir, `processed_${file}`);
        
        const recordCount = await this.processFile(inputPath, outputPath);
        processedFiles.push({
          inputFile: file,
          outputFile: `processed_${file}`,
          recordCount
        });
      }

      // Generate final report
      await this.generateReport(processedFiles);
      
    } catch (error) {
      console.error('Processing failed:', error);
      process.exit(1);
    }
  }

  createSampleData() {
    const sampleData = [
      { id: 1, name: 'John Doe', email: 'john@company.com', age: 30, salary: 75000, department: 'Engineering', join_date: '2020-03-15', status: 'active' },
      { id: 2, name: 'Jane Smith', email: 'jane@company.com', age: 28, salary: 65000, department: 'Marketing', join_date: '2021-06-20', status: 'active' },
      { id: 3, name: 'Bob Johnson', email: 'bob@company.com', age: 35, salary: 85000, department: 'Engineering', join_date: '2019-01-10', status: 'active' },
      { id: 4, name: 'Alice Brown', email: 'alice@company.com', age: 32, salary: 70000, department: 'Sales', join_date: '2020-08-05', status: 'inactive' },
      { id: 5, name: 'Charlie Wilson', email: 'charlie@company.com', age: 29, salary: 60000, department: 'Support', join_date: '2022-02-12', status: 'active' }
    ];

    fs.mkdirSync(this.inputDir, { recursive: true });
    const csvContent = [
      'id,name,email,age,salary,department,join_date,status',
      ...sampleData.map(row => `${row.id},"${row.name}",${row.email},${row.age},${row.salary},${row.department},${row.join_date},${row.status}`)
    ].join('\n');

    fs.writeFileSync(path.join(this.inputDir, 'employees.csv'), csvContent);
    console.log('Sample data created at', path.join(this.inputDir, 'employees.csv'));
  }
}

// Load configuration or use defaults
const config = {
  batchSize: 100,
  maxConcurrency: 10
};

// Run processor
const processor = new DataProcessor(config);
processor.run().then(() => {
  console.log('Data processing completed successfully');
}).catch((error) => {
  console.error('Data processing failed:', error);
  process.exit(1);
});