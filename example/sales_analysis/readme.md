# Run the job with file upload

```bash
# Basic run with file upload
rnx run --upload=sales_analysis.py python3 sales_analysis.py

# With resource limits
rnx run --upload=sales_analysis.py --max-cpu=50 --max-memory=512 python3 sales_analysis.py

# With network isolation (no external access)
rnx run --upload=sales_analysis.py --network=none python3 sales_analysis.py
```