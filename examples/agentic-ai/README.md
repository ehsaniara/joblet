# Agentic AI Foundation Examples

Examples demonstrating how to use Joblet for agentic AI systems, LLM workflows, and intelligent agent coordination.

## 🤖 Examples Overview

| Example | Files | Description | Complexity | Resources |
|---------|-------|-------------|------------|-----------|
| [LLM Inference Service](#llm-inference-service) | `llm_inference.py` | Language model inference with caching | Beginner | 2GB RAM |
| [Multi-Agent System](#multi-agent-system) | `multi_agent_system.py` | Coordinated agent workflows | Intermediate | 4GB RAM |
| [RAG System](#rag-system) | `rag_system.py` | Retrieval-Augmented Generation | Intermediate | 3GB RAM |
| [Distributed Training](#distributed-training) | `distributed_training.py` | Distributed model training simulation | Advanced | 6GB RAM |
| [Complete Demo Suite](#complete-demo-suite) | `run_demos.sh` | Automated execution of all AI examples | All Levels | 8GB RAM |

## 🚀 Quick Start

### Run All Demos (Recommended)
```bash
# Execute complete AI demo suite
./run_demos.sh
```

### Prerequisites (handled automatically by demo script)
```bash
# Create volumes for AI workloads
rnx volume create ai-cache --size=1GB --type=filesystem
rnx volume create ai-outputs --size=2GB --type=filesystem
rnx volume create ai-metrics --size=512MB --type=filesystem
rnx volume create ai-models --size=4GB --type=filesystem
```

### Dependencies
All AI/ML dependencies are listed in `requirements.txt`:
```bash
# Install locally (optional)
pip install -r requirements.txt
```

## 🧠 LLM Inference Service

Language model inference service with intelligent caching and performance metrics.

### Files Included
- **`llm_inference.py`**: Complete LLM inference service with caching and metrics

### Manual Execution
```bash
# Deploy LLM inference service
rnx run --upload=llm_inference.py \
       --volume=ai-cache \
       --volume=ai-outputs \
       --volume=ai-metrics \
       --max-memory=2048 \
       --env=DEMO_MODE=true \
       python3 llm_inference.py
```

### Service Features
- **Intelligent Caching**: MD5-based prompt caching for efficiency
- **Batch Processing**: Handle multiple requests in parallel
- **Performance Metrics**: Track requests, tokens, cache hit rates
- **LLM Simulation**: Realistic response generation for demonstration
- **Response Types**: Code generation, analysis, and general Q&A
- **Persistent Storage**: Cache and results saved to volumes

### Expected Output
- Processed inference requests with realistic response times
- Cache hit/miss statistics and performance metrics
- Generated responses for code, analysis, and general queries
- Results saved to `/volumes/ai-outputs/inference_results_*.json`
- Metrics saved to `/volumes/ai-metrics/inference_metrics_*.json`

**Sample LLM Service Code:**
```python
#!/usr/bin/env python3
import json
import time
import hashlib

# Simulate LLM inference with caching
class LLMInferenceService:
    def __init__(self, config):
        self.config = config
        self.cache_dir = '/volumes/ai-cache'
        self.metrics = {'requests_processed': 0, 'cache_hits': 0}
    
    def process_request(self, prompt, model="gpt-3.5-turbo"):
        # Generate cache key and check cache
        cache_key = hashlib.md5(f"{prompt}:{model}".encode()).hexdigest()
        cached = self.get_cached_response(cache_key)
        
        if cached:
            self.metrics['cache_hits'] += 1
            return cached
        
        # Simulate LLM call
        response = self.simulate_llm_call(prompt, model)
        self.cache_response(cache_key, response)
        self.metrics['requests_processed'] += 1
        return response
```

## 🤖 Multi-Agent System

Coordinated multi-agent system with specialized agents for research, analysis, and writing.

### Files Included
- **`multi_agent_system.py`**: Complete multi-agent coordination system

### Manual Execution
```bash
# Run multi-agent coordination workflow
rnx run --upload=multi_agent_system.py \
       --volume=ai-outputs \
       --volume=ai-metrics \
       --max-memory=4096 \
       --max-cpu=200 \
       python3 multi_agent_system.py
```

### Agent Types
- **Researcher Agents**: Conduct research and gather information
- **Analyst Agents**: Perform data analysis and pattern recognition
- **Writer Agents**: Generate content and documentation
- **Coordinator**: Orchestrates task distribution and synchronization

### System Features
- **Parallel Processing**: Agents work concurrently on different tasks
- **Task Distribution**: Automatic assignment based on agent capabilities
- **Performance Metrics**: Track agent utilization and task completion times
- **Result Aggregation**: Combine outputs from multiple agents
- **Comprehensive Logging**: Detailed execution logs and metrics

### Expected Output
- Coordinated workflow execution across 5 specialized agents
- Research findings, analysis results, and generated content
- Agent performance metrics and utilization statistics
- Workflow results saved to `/volumes/ai-outputs/workflow_results_*.json`

**Sample Multi-Agent Code:**
```python
class MultiAgentSystem:
    def __init__(self, config):
        self.agents = {}
        self.task_queue = []
        self.initialize_agents()
    
    def assign_task(self, task):
        suitable_agents = self.find_suitable_agents(task)
        selected_agent = min(suitable_agents, key=lambda a: len(a.task_history))
        return selected_agent.process_task(task)
    
    def process_tasks_parallel(self, tasks):
        with ThreadPoolExecutor(max_workers=len(self.agents)) as executor:
            futures = []
            for task in tasks:
                agent, assigned_task = self.assign_task(task)
                future = executor.submit(agent.process_task, assigned_task)
                futures.append(future)
            
            results = [future.result() for future in futures]
        return results
```

## 🔍 RAG System

Retrieval-Augmented Generation system with vector database and semantic search.

### Files Included
- **`rag_system.py`**: Complete RAG implementation with vector database

### Manual Execution
```bash
# Run RAG system with knowledge base
rnx run --upload=rag_system.py \
       --volume=ai-cache \
       --volume=ai-outputs \
       --volume=ai-metrics \
       --max-memory=3072 \
       python3 rag_system.py
```

### RAG Features
- **Vector Database**: Simulated vector storage with similarity search
- **Knowledge Base**: Pre-loaded with AI/ML documentation
- **Semantic Search**: Retrieve relevant context for queries
- **Response Generation**: Context-aware response generation
- **Caching System**: Cache both searches and generated responses
- **Source Attribution**: Track and cite source documents

### Knowledge Domains
- Machine Learning deployment strategies
- AI ethics and responsible development
- Vector databases and similarity search
- Large language model fine-tuning
- Agentic AI system architecture

### Expected Output
- Contextual responses to AI/ML questions
- Source document citations and similarity scores
- Cache performance statistics
- Results saved to `/volumes/ai-outputs/rag_demo_results_*.json`

**Sample RAG Code:**
```python
class RAGSystem:
    def __init__(self, config):
        self.vector_db = VectorDatabase(embedding_dim=384)
        self.knowledge_base = self.initialize_knowledge_base()
    
    def query(self, query, top_k=3):
        # Retrieve relevant context
        context = self.vector_db.search(query, top_k=top_k)
        
        # Generate response using context
        response = self.generate_response(query, context)
        
        return {
            'query': query,
            'response': response,
            'sources': [doc['metadata'] for doc in context],
            'context_used': len(context)
        }
```

## 📊 Distributed Training

Distributed machine learning training simulation with multiple workers.

### Files Included
- **`distributed_training.py`**: Distributed training coordinator and workers

### Manual Execution
```bash
# Run distributed training simulation
rnx run --upload=distributed_training.py \
       --volume=ai-models \
       --volume=ai-metrics \
       --max-memory=6144 \
       --max-cpu=300 \
       --env=NUM_WORKERS=4 \
       python3 distributed_training.py
```

### Training Features
- **Multi-Worker Coordination**: 4 parallel training workers
- **Parameter Synchronization**: Simulated gradient averaging
- **Performance Monitoring**: Real-time metrics collection
- **Early Stopping**: Automatic termination when target accuracy reached
- **Comprehensive Reporting**: Training statistics and model artifacts
- **Resource Efficiency**: Optimized memory and CPU usage

### Expected Output
- Training progress across multiple epochs and workers
- Convergence metrics and model performance statistics
- Worker utilization and synchronization overhead metrics
- Model artifacts and training results saved to volumes

**Sample Distributed Training Code:**
```python
class DistributedTrainingCoordinator:
    def __init__(self, config):
        self.num_workers = config.get('num_workers', 4)
        self.workers = [TrainingWorker(i, config) for i in range(self.num_workers)]
        self.global_metrics = {'training_id': str(uuid.uuid4())}
    
    def train_epoch_parallel(self, epoch_num):
        with ThreadPoolExecutor(max_workers=self.num_workers) as executor:
            futures = []
            for worker in self.workers:
                future = executor.submit(worker.train_epoch, epoch_num)
                futures.append(future)
            
            epoch_results = [future.result() for future in futures]
        
        # Synchronize parameters across workers
        return self.synchronize_workers(epoch_results)
```

## 🔄 Complete Demo Suite

Execute all agentic AI examples with a single command.

### Files Included
- **`run_demos.sh`**: Master AI demo script
- **`requirements.txt`**: All AI/ML dependencies

### What It Runs
1. **LLM Inference**: Processes sample prompts with caching and metrics
2. **Multi-Agent System**: Coordinates 5 agents across research, analysis, and writing tasks
3. **RAG System**: Demonstrates semantic search and context-aware generation
4. **Distributed Training**: Simulates distributed ML training with 4 workers
5. **AI Pipeline**: End-to-end pipeline with data preparation, training, and evaluation

### Execution
```bash
# Run complete AI demo suite
./run_demos.sh
```

### Demo Flow
1. Creates required volumes automatically
2. Executes LLM inference with sample prompts
3. Runs multi-agent workflow with task coordination
4. Demonstrates RAG system with knowledge base queries
5. Simulates distributed training across multiple workers
6. Executes AI pipeline stages (data prep, training, evaluation)
7. Provides comprehensive result locations and inspection commands

## 📁 Demo Results

After running the demos, check results in the following locations:

### LLM Inference Results
```bash
# View inference results and metrics
rnx run --volume=ai-outputs cat /volumes/ai-outputs/inference_results_*.json
rnx run --volume=ai-metrics cat /volumes/ai-metrics/inference_metrics_*.json
```

### Multi-Agent Workflow Results
```bash
# View agent coordination results
rnx run --volume=ai-outputs cat /volumes/ai-outputs/workflow_results_*.json
rnx run --volume=ai-metrics cat /volumes/ai-metrics/agent_metrics_*.json
```

### RAG System Results
```bash
# View RAG responses and knowledge base queries
rnx run --volume=ai-outputs cat /volumes/ai-outputs/rag_demo_results_*.json
rnx run --volume=ai-cache ls -la /volumes/ai-cache/
```

### Distributed Training Results
```bash
# View training results and model artifacts
rnx run --volume=ai-models cat /volumes/ai-models/distributed_training_*.json
rnx run --volume=ai-metrics cat /volumes/ai-metrics/training_metrics_*.json
```

### AI Pipeline Results
```bash
# View complete pipeline results
rnx run --volume=ai-outputs cat /volumes/ai-outputs/pipeline/evaluation_results.json
rnx run --volume=ai-models cat /volumes/ai-models/pipeline/model_info.json
```

## 🎯 Best Practices Demonstrated

### AI/ML Workflow Patterns
- **Caching Strategies**: Intelligent prompt caching reduces redundant LLM calls
- **Distributed Processing**: Multi-agent and distributed training patterns
- **Context Management**: RAG system demonstrates effective context retrieval and usage
- **Resource Optimization**: Appropriate memory and CPU limits for different AI workloads

### Production Readiness
- **Metrics Collection**: Comprehensive performance monitoring across all components
- **Error Handling**: Robust error recovery and graceful degradation
- **Scalability**: Patterns for scaling from single agents to distributed systems
- **Persistence**: Proper use of volumes for model artifacts, cache, and results

### Agentic AI Architecture
- **Agent Specialization**: Different agent types for different capabilities
- **Task Orchestration**: Coordination patterns for complex multi-step workflows
- **Knowledge Integration**: RAG patterns for incorporating external knowledge
- **Performance Monitoring**: Real-time metrics for agent and system performance

## 🚀 Next Steps

1. **Connect Real LLMs**: Replace simulation with actual OpenAI, Anthropic, or local model calls
2. **Scale Agent Networks**: Add more specialized agents and coordination patterns
3. **Enhance RAG System**: Connect to real vector databases (Pinecone, Weaviate, Chroma)
4. **Production Deployment**: Add authentication, rate limiting, and comprehensive monitoring
5. **Integration**: Connect with existing AI/ML infrastructure and data pipelines
6. **Advanced Patterns**: Implement agent memory, learning, and adaptation capabilities

## 📊 Monitoring Your AI Jobs

```bash
# Monitor AI job execution in real-time
rnx monitor

# Check job status and resource usage
rnx list

# View specific AI job logs
rnx log <job-id>

# Monitor AI volume usage
rnx volume list
```

## 📚 Additional Resources

- [OpenAI API Documentation](https://platform.openai.com/docs/)
- [Anthropic Claude API](https://docs.anthropic.com/)
- [LangChain Documentation](https://python.langchain.com/)
- [Transformers Library](https://huggingface.co/docs/transformers/)
- [Vector Database Comparison](https://www.pinecone.io/learn/vector-database/)
- [Joblet Documentation](../../docs/) - Configuration and advanced usage