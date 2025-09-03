# Runtime Advanced Scenarios

This document covers advanced deployment patterns, enterprise scenarios, and sophisticated runtime management techniques
for production environments.

## 🏢 Enterprise Deployment Patterns

### Multi-Environment Runtime Promotion

```bash
#!/bin/bash
# runtime_promotion.sh - Promote runtimes through environments

RUNTIME="python-3.11-ml"
ENVIRONMENTS=("staging" "prod-eu" "prod-us" "prod-asia")
ARTIFACT_REGISTRY="https://artifacts.company.com/joblet-runtimes"

# Download certified runtime from artifact registry
curl -H "Authorization: Bearer $REGISTRY_TOKEN" \
     "$ARTIFACT_REGISTRY/$RUNTIME-runtime.zip" \
     -o "$RUNTIME-runtime.zip"

# Deploy to all environments
for env in "${ENVIRONMENTS[@]}"; do
    echo "🚀 Deploying $RUNTIME to $env..."
    
    # Get environment host list
    hosts=$(kubectl get nodes -l environment=$env --no-headers | awk '{print $1}')
    
    for host in $hosts; do
        scp "$RUNTIME-runtime.zip" admin@$host:/tmp/
        ssh admin@$host "sudo unzip /tmp/$RUNTIME-runtime.zip -d /opt/joblet/runtimes/"
        
        # Verify deployment
        ssh admin@$host "rnx runtime test $RUNTIME" || {
            echo "❌ Failed to deploy $RUNTIME on $host"
            exit 1
        }
    done
    
    echo "✅ $env deployment complete"
done
```

### Blue-Green Runtime Deployments

```bash
#!/bin/bash
# blue_green_runtime.sh - Zero-downtime runtime updates

RUNTIME_NAME="python-3.11-ml"
NEW_RUNTIME_FILE="python-3.11-ml-v2.0-runtime.zip"
HEALTH_CHECK_SCRIPT="runtime_health_check.py"

deploy_runtime_blue_green() {
    local host=$1
    local runtime_file=$2
    
    echo "🔄 Starting blue-green deployment on $host"
    
    # Step 1: Deploy new runtime with versioned name
    ssh admin@$host "sudo unzip /tmp/$runtime_file -d /opt/joblet/runtimes/"
    
    # Step 2: Health check new runtime
    ssh admin@$host "rnx run --runtime=python-3.11-ml-v2.0 python $HEALTH_CHECK_SCRIPT" || {
        echo "❌ Health check failed on $host"
        return 1
    }
    
    # Step 3: Update symlink for seamless cutover (if using symlinks)
    ssh admin@$host "sudo ln -sfn /opt/joblet/runtimes/python/python-3.11-ml-v2.0 /opt/joblet/runtimes/python/python-3.11-ml"
    
    # Step 4: Verify production traffic
    ssh admin@$host "rnx run --runtime=python-3.11-ml python -c 'print(\"✅ Production ready\")'"
    
    echo "✅ Blue-green deployment complete on $host"
}

# Deploy to production cluster
PROD_HOSTS=("prod-web-01" "prod-web-02" "prod-worker-01" "prod-worker-02")
for host in "${PROD_HOSTS[@]}"; do
    deploy_runtime_blue_green "$host" "$NEW_RUNTIME_FILE"
done
```

## 🔄 CI/CD Integration Patterns

### GitHub Actions Enterprise Workflow

```yaml
name: Runtime Build & Deploy Pipeline
on:
  push:
    paths:
      - 'runtimes/**'
      - '.github/workflows/runtime-deploy.yml'

env:
  ARTIFACT_REGISTRY: ${{ secrets.ARTIFACT_REGISTRY_URL }}
  REGISTRY_TOKEN: ${{ secrets.REGISTRY_TOKEN }}

jobs:
  build-runtime:
    runs-on: [ self-hosted, linux, build-cluster ]
    strategy:
      matrix:
        runtime: [ python-3.11-ml, node-18, java-17 ]

    steps:
      - uses: actions/checkout@v4

      - name: Build Runtime
        run: |
          sudo ./runtimes/${{ matrix.runtime }}/setup_${{ matrix.runtime }}.sh

      - name: Security Scan
        run: |
          # Scan runtime for vulnerabilities
          trivy fs /opt/joblet/runtimes/ --format table --exit-code 1

      - name: Package Verification
        run: |
          # Verify package integrity
          zip -T /tmp/runtime-deployments/${{ matrix.runtime }}-runtime.zip

      - name: Upload to Registry
        run: |
          curl -X PUT \
               -H "Authorization: Bearer ${{ env.REGISTRY_TOKEN }}" \
               -F "file=@/tmp/runtime-deployments/${{ matrix.runtime }}-runtime.zip" \
               "${{ env.ARTIFACT_REGISTRY }}/${{ matrix.runtime }}-runtime.zip"

  deploy-staging:
    needs: build-runtime
    runs-on: ubuntu-latest
    environment: staging

    steps:
      - name: Deploy to Staging
        run: |
          for runtime in python-3.11-ml node-18 java-17; do
            for host in ${{ secrets.STAGING_HOSTS }}; do
              # Download from registry
              curl -H "Authorization: Bearer ${{ env.REGISTRY_TOKEN }}" \
                   "${{ env.ARTIFACT_REGISTRY }}/${runtime}-runtime.zip" \
                   -o "${runtime}-runtime.zip"

              # Deploy to staging host
              scp "${runtime}-runtime.zip" admin@${host}:/tmp/
              ssh admin@${host} "sudo unzip /tmp/${runtime}-runtime.zip -d /opt/joblet/runtimes/"

              # Integration test
              ssh admin@${host} "rnx runtime test ${runtime}"
            done
          done

  deploy-production:
    needs: deploy-staging
    runs-on: ubuntu-latest
    environment: production

    steps:
      - name: Production Deployment
        run: |
          # Production deployment with approval gate
          echo "🚀 Starting production deployment..."
          ./scripts/production_runtime_deploy.sh
```

### Jenkins Pipeline Enterprise

```groovy
pipeline {
    agent { label 'linux-build' }

    parameters {
        choice(name: 'RUNTIME',
                choices: ['python-3.11-ml', 'node-18', 'java-17', 'all'],
                description: 'Runtime to deploy')
        choice(name: 'ENVIRONMENT',
                choices: ['staging', 'production'],
                description: 'Target environment')
    }

    stages {
        stage('Build Runtime') {
            when { params.RUNTIME != 'all' }
            steps {
                sh """
                    sudo ./runtimes/${params.RUNTIME}/setup_${params.RUNTIME}.sh
                    zip -T /tmp/runtime-deployments/${params.RUNTIME}-runtime.zip
                """
            }
        }

        stage('Build All Runtimes') {
            when { params.RUNTIME == 'all' }
            parallel {
                stage('Python ML') {
                    steps {
                        sh 'sudo ./runtimes/python-3.11-ml/setup_python_3_11_ml.sh'
                    }
                }
                stage('Node.js') {
                    steps {
                        sh 'sudo ./runtimes/node-18/setup_node_18.sh'
                    }
                }
                stage('Java') {
                    steps {
                        sh 'sudo ./runtimes/java-17/setup_java_17.sh'
                    }
                }
            }
        }

        stage('Security Scan') {
            steps {
                sh '''
                    # Vulnerability scanning
                    for runtime_zip in /tmp/runtime-deployments/*.zip; do
                        echo "Scanning $runtime_zip"
                        unzip -q "$runtime_zip" -d "/tmp/scan-$(basename $runtime_zip)"
                        trivy fs "/tmp/scan-$(basename $runtime_zip)" --exit-code 1
                    done
                '''
            }
        }

        stage('Deploy to Staging') {
            when { params.ENVIRONMENT == 'staging' }
            steps {
                script {
                    def hosts = env.STAGING_HOSTS.split(',')
                    def runtimes = params.RUNTIME == 'all' ?
                            ['python-3.11-ml', 'node-18', 'java-17'] :
                            [params.RUNTIME]

                    hosts.each { host ->
                        runtimes.each { runtime ->
                            sh """
                                scp /tmp/runtime-deployments/${runtime}-runtime.zip admin@${host}:/tmp/
                                ssh admin@${host} "sudo unzip /tmp/${runtime}-runtime.zip -d /opt/joblet/runtimes/"
                                ssh admin@${host} "rnx runtime test ${runtime}"
                            """
                        }
                    }
                }
            }
        }

        stage('Production Approval') {
            when { params.ENVIRONMENT == 'production' }
            steps {
                input message: 'Deploy to Production?',
                        ok: 'Deploy',
                        parameters: [
                                string(name: 'APPROVER', description: 'Your name for audit trail')
                        ]
            }
        }

        stage('Deploy to Production') {
            when { params.ENVIRONMENT == 'production' }
            steps {
                sh './scripts/production_runtime_deploy.sh ${params.RUNTIME}'
            }
        }
    }

    post {
        success {
            slackSend channel: '#deployments',
                    message: "✅ Runtime deployment successful: ${params.RUNTIME} → ${params.ENVIRONMENT}"
        }
        failure {
            slackSend channel: '#alerts',
                    message: "❌ Runtime deployment failed: ${params.RUNTIME} → ${params.ENVIRONMENT}"
        }
    }
}
```

## 📊 Monitoring and Observability

### Runtime Deployment Monitoring

```bash
#!/bin/bash
# runtime_monitor.sh - Monitor runtime health across fleet

RUNTIMES=("python-3.11-ml" "node-18" "java-17")
HOSTS_FILE="production_hosts.txt"
METRICS_ENDPOINT="http://prometheus.company.com:9090/api/v1/query"

check_runtime_health() {
    local host=$1
    local runtime=$2
    
    # Test runtime availability
    if ssh admin@$host "timeout 30 rnx runtime test $runtime" >/dev/null 2>&1; then
        echo "runtime_health{host=\"$host\",runtime=\"$runtime\"} 1"
    else
        echo "runtime_health{host=\"$host\",runtime=\"$runtime\"} 0"
        # Alert on failure
        curl -X POST "https://alerts.company.com/webhook" \
             -H "Content-Type: application/json" \
             -d "{\"alert\": \"Runtime $runtime failed on $host\"}"
    fi
    
    # Check runtime version
    version=$(ssh admin@$host "rnx runtime info $runtime | grep Version" 2>/dev/null || echo "unknown")
    echo "runtime_version{host=\"$host\",runtime=\"$runtime\",version=\"$version\"} 1"
}

# Check all hosts and runtimes
while IFS= read -r host; do
    for runtime in "${RUNTIMES[@]}"; do
        check_runtime_health "$host" "$runtime"
    done
done < "$HOSTS_FILE"
```

### Grafana Dashboard Query Examples

```promql
# Runtime availability across fleet
avg(runtime_health) by (runtime)

# Failed runtime deployments
sum(rate(runtime_deployment_failures_total[5m])) by (runtime, host)

# Runtime deployment time
histogram_quantile(0.95, rate(runtime_deployment_duration_seconds_bucket[5m]))

# Runtime package size tracking
runtime_package_size_bytes by (runtime, version)
```

## 🔐 Security Hardening

### Runtime Package Signing

```bash
#!/bin/bash
# sign_runtime.sh - Sign runtime packages for security

RUNTIME_PACKAGE="$1"
PRIVATE_KEY="/secure/runtime-signing-key.pem"
PUBLIC_KEY="/secure/runtime-signing-key.pub"

# Generate signature
openssl dgst -sha256 -sign "$PRIVATE_KEY" \
         -out "${RUNTIME_PACKAGE}.sig" \
         "$RUNTIME_PACKAGE"

# Create manifest
cat > "${RUNTIME_PACKAGE}.manifest" <<EOF
{
    "package": "$(basename $RUNTIME_PACKAGE)",
    "sha256": "$(sha256sum $RUNTIME_PACKAGE | cut -d' ' -f1)",
    "signature": "$(base64 -w0 ${RUNTIME_PACKAGE}.sig)",
    "signed_by": "$(whoami)@$(hostname)",
    "signed_at": "$(date -Iseconds)",
    "public_key": "$(cat $PUBLIC_KEY | base64 -w0)"
}
EOF

echo "✅ Package signed: ${RUNTIME_PACKAGE}.manifest"
```

### Runtime Package Verification

```bash
#!/bin/bash
# verify_runtime.sh - Verify runtime package signatures

RUNTIME_PACKAGE="$1"
MANIFEST_FILE="${RUNTIME_PACKAGE}.manifest"
TRUSTED_KEYS="/etc/joblet/trusted-keys/"

# Extract signature and verify
public_key_hash=$(jq -r '.public_key' "$MANIFEST_FILE" | base64 -d | sha256sum | cut -d' ' -f1)

if [ -f "$TRUSTED_KEYS/$public_key_hash.pub" ]; then
    echo "✅ Public key is trusted"
    
    # Verify signature
    jq -r '.signature' "$MANIFEST_FILE" | base64 -d > /tmp/runtime.sig
    jq -r '.public_key' "$MANIFEST_FILE" | base64 -d > /tmp/public.key
    
    if openssl dgst -sha256 -verify /tmp/public.key \
                    -signature /tmp/runtime.sig \
                    "$RUNTIME_PACKAGE"; then
        echo "✅ Package signature valid"
        # Deploy with confidence
        sudo unzip "$RUNTIME_PACKAGE" -d /opt/joblet/runtimes/
    else
        echo "❌ Package signature invalid - deployment blocked"
        exit 1
    fi
else
    echo "❌ Public key not trusted - deployment blocked"
    exit 1
fi
```

## 📈 Performance Optimization

### Parallel Deployment Strategy

```bash
#!/bin/bash
# parallel_deploy.sh - Deploy runtimes in parallel

RUNTIME_PACKAGE="$1"
HOSTS_FILE="production_hosts.txt"
MAX_PARALLEL=10

deploy_to_host() {
    local host=$1
    local package=$2
    
    echo "🚀 Deploying to $host..."
    scp "$package" admin@$host:/tmp/ && \
    ssh admin@$host "sudo unzip /tmp/$(basename $package) -d /opt/joblet/runtimes/" && \
    ssh admin@$host "rnx runtime test $(basename $package .zip | sed 's/-runtime$//')" && \
    echo "✅ $host deployment complete"
}

# Export function for parallel execution
export -f deploy_to_host

# Deploy in parallel with controlled concurrency
cat "$HOSTS_FILE" | xargs -I {} -P "$MAX_PARALLEL" bash -c 'deploy_to_host "$@"' _ {} "$RUNTIME_PACKAGE"
```

## 🎯 Related Documentation

- [Runtime System Overview](RUNTIME_SYSTEM.md)
- [Basic Runtime Deployment](RUNTIME_DEPLOYMENT.md)
- [Security Configuration](SECURITY.md)
- [Performance Tuning](docs/PERFORMANCE.md)
- [Custom Runtime Creation](docs/CUSTOM_RUNTIMES.md)