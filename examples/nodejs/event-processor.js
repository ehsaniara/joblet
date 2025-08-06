const EventEmitter = require('events');
const fs = require('fs');
const path = require('path');

class EventProcessor extends EventEmitter {
  constructor(config) {
    super();
    this.config = config;
    this.outputDir = '/volumes/nodejs-projects/events';
    this.metricsDir = '/volumes/nodejs-projects/metrics';
    
    // Create directories if volumes exist
    if (fs.existsSync('/volumes/nodejs-projects')) {
      fs.mkdirSync(this.outputDir, { recursive: true });
      fs.mkdirSync(this.metricsDir, { recursive: true });
    }
    
    this.metrics = {
      eventsProcessed: 0,
      eventsPerSecond: 0,
      errors: 0,
      startTime: Date.now()
    };
    
    this.setupMetricsReporting();
  }

  setupMetricsReporting() {
    this.metricsInterval = setInterval(() => {
      const now = Date.now();
      const elapsed = (now - this.metrics.startTime) / 1000;
      this.metrics.eventsPerSecond = Math.round(this.metrics.eventsProcessed / elapsed);
      
      console.log(`📊 Metrics: ${this.metrics.eventsProcessed} events, ${this.metrics.eventsPerSecond} events/sec, ${this.metrics.errors} errors`);
      
      // Save metrics if volume exists
      if (fs.existsSync('/volumes/nodejs-projects')) {
        const metricsFile = path.join(this.metricsDir, 'real-time-metrics.json');
        fs.writeFileSync(metricsFile, JSON.stringify({
          ...this.metrics,
          timestamp: new Date().toISOString()
        }, null, 2));
      }
      
    }, 5000);
  }

  async processEvent(event) {
    try {
      // Event processing logic
      const processed = {
        id: event.id,
        type: event.type,
        timestamp: new Date().toISOString(),
        data: this.transformEventData(event.data),
        metadata: {
          source: event.source,
          processedBy: 'joblet-event-processor',
          processingTime: Date.now()
        }
      };

      // Route to appropriate handler
      switch (event.type) {
        case 'user.created':
          await this.handleUserCreated(processed);
          break;
        case 'order.placed':
          await this.handleOrderPlaced(processed);
          break;
        case 'payment.processed':
          await this.handlePaymentProcessed(processed);
          break;
        default:
          await this.handleGenericEvent(processed);
      }

      this.metrics.eventsProcessed++;
      this.emit('eventProcessed', processed);
      
    } catch (error) {
      this.metrics.errors++;
      this.emit('error', error, event);
    }
  }

  transformEventData(data) {
    // Common data transformations
    if (data.email) {
      data.email = data.email.toLowerCase();
    }
    
    if (data.timestamp) {
      data.timestamp = new Date(data.timestamp).toISOString();
    }
    
    // Sanitize sensitive data
    if (data.password) {
      delete data.password;
    }
    
    return data;
  }

  async handleUserCreated(event) {
    console.log(`👤 User created: ${event.data.email}`);
    
    if (fs.existsSync('/volumes/nodejs-projects')) {
      // Save to user events log
      const userEventsFile = path.join(this.outputDir, 'user-events.jsonl');
      fs.appendFileSync(userEventsFile, JSON.stringify(event) + '\n');
    }
    
    // Trigger welcome email workflow
    this.emit('triggerWorkflow', {
      type: 'send_welcome_email',
      userId: event.data.id,
      email: event.data.email
    });
  }

  async handleOrderPlaced(event) {
    console.log(`🛒 Order placed: ${event.data.orderId} - $${event.data.amount}`);
    
    if (fs.existsSync('/volumes/nodejs-projects')) {
      // Save to orders log
      const ordersFile = path.join(this.outputDir, 'order-events.jsonl');
      fs.appendFileSync(ordersFile, JSON.stringify(event) + '\n');
    }
    
    // Update inventory
    this.emit('triggerWorkflow', {
      type: 'update_inventory',
      orderId: event.data.orderId,
      items: event.data.items
    });
  }

  async handlePaymentProcessed(event) {
    console.log(`💳 Payment processed: ${event.data.paymentId} - $${event.data.amount}`);
    
    if (fs.existsSync('/volumes/nodejs-projects')) {
      // Save to payments log
      const paymentsFile = path.join(this.outputDir, 'payment-events.jsonl');
      fs.appendFileSync(paymentsFile, JSON.stringify(event) + '\n');
    }
    
    // Send confirmation
    this.emit('triggerWorkflow', {
      type: 'send_payment_confirmation',
      paymentId: event.data.paymentId,
      userId: event.data.userId
    });
  }

  async handleGenericEvent(event) {
    console.log(`📝 Generic event: ${event.type}`);
    
    if (fs.existsSync('/volumes/nodejs-projects')) {
      const genericFile = path.join(this.outputDir, 'generic-events.jsonl');
      fs.appendFileSync(genericFile, JSON.stringify(event) + '\n');
    }
  }

  start() {
    console.log('🚀 Event Processor Started');
    
    // Simulate event stream (in real app, connect to Kafka, Redis, etc.)
    this.simulateEventStream();
    
    // Setup error handling
    this.on('error', (error, event) => {
      console.error('❌ Event processing error:', error.message);
      
      if (fs.existsSync('/volumes/nodejs-projects')) {
        const errorLog = {
          timestamp: new Date().toISOString(),
          error: error.message,
          event: event,
          stack: error.stack
        };
        
        const errorFile = path.join(this.outputDir, 'error-events.jsonl');
        fs.appendFileSync(errorFile, JSON.stringify(errorLog) + '\n');
      }
    });
  }

  simulateEventStream() {
    const eventTypes = [
      'user.created',
      'order.placed',
      'payment.processed',
      'product.viewed',
      'cart.updated'
    ];

    this.eventInterval = setInterval(() => {
      const event = {
        id: `event_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
        type: eventTypes[Math.floor(Math.random() * eventTypes.length)],
        source: 'demo-app',
        data: this.generateSampleEventData(),
        timestamp: new Date().toISOString()
      };

      this.processEvent(event);
    }, 100); // Process event every 100ms
  }

  generateSampleEventData() {
    return {
      id: `user_${Math.floor(Math.random() * 10000)}`,
      email: `user${Math.floor(Math.random() * 1000)}@example.com`,
      amount: Math.floor(Math.random() * 1000) + 10,
      orderId: `order_${Date.now()}`,
      paymentId: `payment_${Date.now()}`,
      items: [
        { id: 'item1', quantity: Math.floor(Math.random() * 5) + 1 },
        { id: 'item2', quantity: Math.floor(Math.random() * 3) + 1 }
      ]
    };
  }

  stop() {
    console.log('🛑 Stopping event processor...');
    
    if (this.eventInterval) clearInterval(this.eventInterval);
    if (this.metricsInterval) clearInterval(this.metricsInterval);
    
    // Save final metrics
    const finalMetrics = {
      ...this.metrics,
      endTime: Date.now(),
      shutdownReason: 'manual_stop'
    };
    
    if (fs.existsSync('/volumes/nodejs-projects')) {
      const metricsFile = path.join(this.metricsDir, 'final-metrics.json');
      fs.writeFileSync(metricsFile, JSON.stringify(finalMetrics, null, 2));
    }
    
    console.log('Event processor stopped');
  }
}

// Start event processor
const config = {
  batchSize: 100,
  maxConcurrency: 10,
  retryAttempts: 3
};

const processor = new EventProcessor(config);
processor.start();

// Graceful shutdown
process.on('SIGTERM', () => {
  processor.stop();
  process.exit(0);
});

// Stop after 60 seconds in demo mode
if (process.env.DEMO_MODE) {
  setTimeout(() => {
    console.log('Demo time limit reached, stopping...');
    processor.stop();
    process.exit(0);
  }, 60000);
}