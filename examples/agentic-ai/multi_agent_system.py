#!/usr/bin/env python3
import json
import time
import uuid
import os
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
import logging
import threading

# Setup logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')

class Agent:
    def __init__(self, agent_id, agent_type, capabilities):
        self.agent_id = agent_id
        self.agent_type = agent_type
        self.capabilities = capabilities
        self.logger = logging.getLogger(f"Agent-{agent_id}")
        self.status = "idle"
        self.task_history = []
        
    def process_task(self, task):
        """Process a task assigned to this agent"""
        self.status = "working"
        self.logger.info(f"Starting task: {task['task_id']}")
        
        start_time = time.time()
        
        try:
            # Simulate task processing based on agent type
            result = self._execute_task(task)
            
            # Record task completion
            task_record = {
                'task_id': task['task_id'],
                'started_at': datetime.fromtimestamp(start_time).isoformat(),
                'completed_at': datetime.now().isoformat(),
                'duration': time.time() - start_time,
                'status': 'completed',
                'result': result
            }
            
            self.task_history.append(task_record)
            self.status = "idle"
            self.logger.info(f"Completed task: {task['task_id']} in {task_record['duration']:.2f}s")
            
            return task_record
            
        except Exception as e:
            self.logger.error(f"Task failed: {task['task_id']} - {str(e)}")
            self.status = "error"
            raise

    def _execute_task(self, task):
        """Execute task based on agent type"""
        if self.agent_type == "researcher":
            return self._research_task(task)
        elif self.agent_type == "analyst":
            return self._analysis_task(task)
        elif self.agent_type == "writer":
            return self._writing_task(task)
        else:
            return self._generic_task(task)

    def _research_task(self, task):
        """Simulate research task"""
        self.logger.info("Conducting research...")
        time.sleep(2)  # Simulate research time
        
        query = task.get('query', 'general research')
        return {
            'type': 'research_results',
            'query': query,
            'findings': [
                f"Research finding 1 for '{query}': Current market trends show significant growth",
                f"Research finding 2 for '{query}': Key technologies are emerging in this space",
                f"Research finding 3 for '{query}': Competitive landscape analysis reveals opportunities"
            ],
            'sources': [
                "Academic papers database",
                "Industry reports",
                "Market research data"
            ],
            'confidence': 0.85
        }

    def _analysis_task(self, task):
        """Simulate analysis task"""
        self.logger.info("Performing analysis...")
        time.sleep(1.5)  # Simulate analysis time
        
        data = task.get('data', {})
        return {
            'type': 'analysis_results',
            'input_data': data,
            'insights': [
                "Pattern identified: Cyclical behavior in data trends",
                "Anomaly detected: Unusual spike in Q3 metrics",
                "Correlation found: Strong relationship between variables A and B"
            ],
            'recommendations': [
                "Increase monitoring frequency during peak periods",
                "Investigate root cause of Q3 anomaly",
                "Leverage A-B correlation for predictive modeling"
            ],
            'confidence': 0.92
        }

    def _writing_task(self, task):
        """Simulate writing task"""
        self.logger.info("Writing content...")
        time.sleep(2.5)  # Simulate writing time
        
        topic = task.get('topic', 'general topic')
        content_type = task.get('content_type', 'report')
        
        return {
            'type': 'written_content',
            'topic': topic,
            'content_type': content_type,
            'content': f"""# {topic.title()} {content_type.title()}

## Executive Summary
This {content_type} provides a comprehensive overview of {topic}, based on recent research and analysis. The findings indicate significant opportunities for improvement and growth.

## Key Findings
1. **Market Opportunity**: There is substantial potential in the {topic} sector
2. **Technical Feasibility**: Current technology supports implementation
3. **Resource Requirements**: Moderate investment needed for successful execution

## Recommendations
- Prioritize immediate implementation of key initiatives
- Establish monitoring and evaluation frameworks
- Develop strategic partnerships for enhanced capabilities

## Conclusion
The evidence strongly supports moving forward with the proposed {topic} initiative. The combination of market opportunity and technical feasibility creates an ideal environment for success.
""",
            'word_count': 150,
            'readability_score': 8.2
        }

    def _generic_task(self, task):
        """Simulate generic task"""
        self.logger.info("Processing generic task...")
        time.sleep(1)
        
        return {
            'type': 'generic_result',
            'task_type': task.get('type', 'unknown'),
            'status': 'completed',
            'message': f"Successfully processed {task.get('type', 'generic')} task"
        }

class MultiAgentSystem:
    def __init__(self, config):
        self.config = config
        self.agents = {}
        self.task_queue = []
        self.completed_tasks = []
        self.system_metrics = {
            'tasks_processed': 0,
            'total_processing_time': 0,
            'agent_utilization': {},
            'start_time': datetime.now().isoformat()
        }
        self.logger = logging.getLogger("MultiAgentSystem")
        self.output_dir = '/volumes/ai-outputs'
        self.metrics_dir = '/volumes/ai-metrics'
        
        # Create directories if volumes exist
        for dir_path in [self.output_dir, self.metrics_dir]:
            if os.path.exists('/volumes'):
                os.makedirs(dir_path, exist_ok=True)
        
        self._initialize_agents()

    def _initialize_agents(self):
        """Initialize agents based on configuration"""
        agent_configs = self.config.get('agents', [])
        
        for agent_config in agent_configs:
            agent = Agent(
                agent_id=agent_config['id'],
                agent_type=agent_config['type'], 
                capabilities=agent_config.get('capabilities', [])
            )
            self.agents[agent.agent_id] = agent
            self.system_metrics['agent_utilization'][agent.agent_id] = {
                'tasks_completed': 0,
                'total_time': 0,
                'average_time': 0
            }
            
        self.logger.info(f"Initialized {len(self.agents)} agents")

    def assign_task(self, task):
        """Assign task to most suitable agent"""
        task['task_id'] = str(uuid.uuid4())
        task['created_at'] = datetime.now().isoformat()
        
        # Find best agent for task
        suitable_agents = self._find_suitable_agents(task)
        if not suitable_agents:
            raise ValueError(f"No suitable agents found for task type: {task.get('type')}")
        
        # Select least busy agent
        selected_agent = min(suitable_agents, key=lambda a: len(a.task_history))
        
        self.logger.info(f"Assigning task {task['task_id']} to agent {selected_agent.agent_id}")
        return selected_agent, task

    def _find_suitable_agents(self, task):
        """Find agents capable of handling the task"""
        task_type = task.get('type', 'generic')
        suitable_agents = []
        
        for agent in self.agents.values():
            if agent.status in ['idle', 'working']:
                if task_type == 'research' and agent.agent_type == 'researcher':
                    suitable_agents.append(agent)
                elif task_type == 'analysis' and agent.agent_type == 'analyst':
                    suitable_agents.append(agent)
                elif task_type == 'writing' and agent.agent_type == 'writer':
                    suitable_agents.append(agent)
                elif task_type == 'generic':
                    suitable_agents.append(agent)
        
        return suitable_agents

    def process_tasks_parallel(self, tasks):
        """Process multiple tasks in parallel"""
        self.logger.info(f"Processing {len(tasks)} tasks in parallel")
        
        with ThreadPoolExecutor(max_workers=len(self.agents)) as executor:
            # Submit tasks
            future_to_task = {}
            for task in tasks:
                try:
                    agent, assigned_task = self.assign_task(task)
                    future = executor.submit(agent.process_task, assigned_task)
                    future_to_task[future] = (agent, assigned_task)
                except ValueError as e:
                    self.logger.error(str(e))
                    continue
            
            # Collect results
            results = []
            for future in as_completed(future_to_task):
                agent, task = future_to_task[future]
                try:
                    result = future.result()
                    results.append(result)
                    
                    # Update metrics
                    self.system_metrics['tasks_processed'] += 1
                    self.system_metrics['total_processing_time'] += result['duration']
                    
                    agent_metrics = self.system_metrics['agent_utilization'][agent.agent_id]
                    agent_metrics['tasks_completed'] += 1
                    agent_metrics['total_time'] += result['duration']
                    agent_metrics['average_time'] = agent_metrics['total_time'] / agent_metrics['tasks_completed']
                    
                except Exception as e:
                    self.logger.error(f"Task execution failed: {str(e)}")
        
        return results

    def run_demo_workflow(self):
        """Run a demonstration workflow with coordinated agents"""
        self.logger.info("Starting multi-agent demo workflow")
        
        # Define a complex workflow
        workflow_tasks = [
            {
                'type': 'research',
                'query': 'artificial intelligence trends 2024',
                'priority': 'high'
            },
            {
                'type': 'research',
                'query': 'machine learning deployment strategies',
                'priority': 'medium'
            },
            {
                'type': 'analysis',
                'data': {'market_size': 50000000, 'growth_rate': 0.25, 'competition': 'high'},
                'analysis_type': 'market_analysis'
            },
            {
                'type': 'analysis',
                'data': {'user_engagement': 0.75, 'retention_rate': 0.68, 'satisfaction': 8.2},
                'analysis_type': 'user_analytics'
            },
            {
                'type': 'writing',
                'topic': 'AI Implementation Strategy',
                'content_type': 'executive_summary'
            },
            {
                'type': 'writing',
                'topic': 'Technical Architecture',
                'content_type': 'technical_document'
            }
        ]
        
        # Process tasks
        start_time = time.time()
        results = self.process_tasks_parallel(workflow_tasks)
        total_time = time.time() - start_time
        
        # Save results
        workflow_result = {
            'workflow_id': str(uuid.uuid4()),
            'completed_at': datetime.now().isoformat(),
            'total_time': total_time,
            'tasks_completed': len(results),
            'results': results,
            'system_metrics': self.system_metrics
        }
        
        if os.path.exists(self.output_dir):
            output_file = os.path.join(self.output_dir, f'workflow_results_{int(time.time())}.json')
            with open(output_file, 'w') as f:
                json.dump(workflow_result, f, indent=2)
            self.logger.info(f"Workflow results saved to {output_file}")
        
        # Save metrics
        if os.path.exists(self.metrics_dir):
            metrics_file = os.path.join(self.metrics_dir, f'agent_metrics_{int(time.time())}.json')
            with open(metrics_file, 'w') as f:
                json.dump(self.system_metrics, f, indent=2)
            self.logger.info(f"System metrics saved to {metrics_file}")
        
        # Display summary
        print("\n" + "="*60)
        print("MULTI-AGENT SYSTEM DEMO SUMMARY")
        print("="*60)
        print(f"Workflow completed in: {total_time:.2f} seconds")
        print(f"Tasks processed: {len(results)}")
        print(f"Active agents: {len(self.agents)}")
        
        print("\nAgent Performance:")
        for agent_id, metrics in self.system_metrics['agent_utilization'].items():
            agent = self.agents[agent_id]
            print(f"  {agent_id} ({agent.agent_type}):")
            print(f"    Tasks completed: {metrics['tasks_completed']}")
            print(f"    Average time: {metrics['average_time']:.2f}s")
            print(f"    Status: {agent.status}")
        
        if self.system_metrics['tasks_processed'] > 0:
            avg_time = self.system_metrics['total_processing_time'] / self.system_metrics['tasks_processed']
            print(f"\nOverall average task time: {avg_time:.2f} seconds")
        
        return workflow_result

if __name__ == "__main__":
    # Configuration for multi-agent system
    config = {
        'agents': [
            {
                'id': 'researcher-001',
                'type': 'researcher',
                'capabilities': ['web_search', 'data_collection', 'source_validation']
            },
            {
                'id': 'researcher-002', 
                'type': 'researcher',
                'capabilities': ['academic_research', 'technical_analysis']
            },
            {
                'id': 'analyst-001',
                'type': 'analyst',
                'capabilities': ['data_analysis', 'statistical_modeling', 'pattern_recognition']
            },
            {
                'id': 'analyst-002',
                'type': 'analyst', 
                'capabilities': ['market_analysis', 'competitive_intelligence']
            },
            {
                'id': 'writer-001',
                'type': 'writer',
                'capabilities': ['technical_writing', 'report_generation', 'content_optimization']
            }
        ],
        'coordination': {
            'max_parallel_tasks': 10,
            'task_timeout': 300,
            'retry_attempts': 3
        }
    }
    
    # Initialize and run system
    system = MultiAgentSystem(config)
    results = system.run_demo_workflow()
    
    print("\nMulti-agent demo completed successfully!")