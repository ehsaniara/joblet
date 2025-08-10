#!/usr/bin/env python3
"""
ML Analysis Example
Demonstrates packaging ML dependencies with the project for python:3.11 runtime
"""
import sys
import os

# Add local lib directory to path for packaged dependencies
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'lib'))

# Now import ML packages from local lib/
try:
    import pandas as pd
    import numpy as np
    import matplotlib.pyplot as plt
    import seaborn as sns
    from sklearn.ensemble import RandomForestClassifier
    from sklearn.model_selection import train_test_split
    from sklearn.metrics import accuracy_score, classification_report
    import requests
except ImportError as e:
    print(f"❌ Import error: {e}")
    print("🔧 Make sure to run: pip install -r requirements.txt --target lib/")
    sys.exit(1)

def main():
    print("🚀 ML Analysis with python:3.11")
    print("=" * 40)
    print(f"Python Version: {sys.version}")
    print(f"Working Directory: {os.getcwd()}")
    print(f"Python Path: {sys.path[0]}")  # Show our lib path
    print()

    # Show package versions
    print("📦 Package Versions (from local lib/):")
    packages = [
        ('pandas', pd),
        ('numpy', np),
        ('matplotlib', plt.matplotlib),
        ('seaborn', sns),
        ('sklearn', __import__('sklearn')),
        ('requests', requests)
    ]

    for name, pkg in packages:
        version = getattr(pkg, '__version__', 'unknown')
        print(f"  ✅ {name}: {version}")
    print()

    # Generate synthetic dataset
    print("📊 Generating ML Dataset...")
    np.random.seed(42)
    n_samples = 1000

    # Create features
    X = np.random.randn(n_samples, 5)
    # Create target with some relationship to features
    y = (X[:, 0] + X[:, 1] - 0.5 * X[:, 2] + np.random.randn(n_samples) * 0.3 > 0).astype(int)

    # Create DataFrame for analysis
    feature_names = [f'feature_{i}' for i in range(X.shape[1])]
    df = pd.DataFrame(X, columns=feature_names)
    df['target'] = y

    print(f"Dataset shape: {df.shape}")
    print("\nDataset statistics:")
    print(df.describe())
    print()

    # Train ML Model
    print("🤖 Training Machine Learning Model...")
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

    # Train Random Forest
    clf = RandomForestClassifier(n_estimators=100, random_state=42)
    clf.fit(X_train, y_train)

    # Make predictions
    y_pred = clf.predict(X_test)
    accuracy = accuracy_score(y_test, y_pred)

    print(f"Model Accuracy: {accuracy:.3f}")
    print("\nClassification Report:")
    print(classification_report(y_test, y_pred))

    # Feature importance
    print("📈 Feature Importance:")
    importance_df = pd.DataFrame({
        'feature': feature_names,
        'importance': clf.feature_importances_
    }).sort_values('importance', ascending=False)
    print(importance_df.to_string(index=False))
    print()

    # Create visualization
    print("📊 Creating Visualizations...")

    # Set style
    plt.style.use('default')
    sns.set_palette("husl")

    # Create subplots
    fig, ((ax1, ax2), (ax3, ax4)) = plt.subplots(2, 2, figsize=(12, 10))
    fig.suptitle('ML Analysis Results', fontsize=16)

    # Plot 1: Feature distributions
    df.hist(column=feature_names[:3], bins=20, ax=ax1, alpha=0.7)
    ax1.set_title('Feature Distributions')

    # Plot 2: Correlation heatmap
    corr_matrix = df[feature_names].corr()
    sns.heatmap(corr_matrix, annot=True, cmap='coolwarm', center=0, ax=ax2)
    ax2.set_title('Feature Correlation Matrix')

    # Plot 3: Feature importance
    ax3.bar(importance_df['feature'], importance_df['importance'])
    ax3.set_title('Feature Importance')
    ax3.tick_params(axis='x', rotation=45)

    # Plot 4: Target distribution
    target_counts = df['target'].value_counts()
    ax4.pie(target_counts.values, labels=[f'Class {i}' for i in target_counts.index], autopct='%1.1f%%')
    ax4.set_title('Target Class Distribution')

    plt.tight_layout()
    plt.savefig('ml_analysis_results.png', dpi=150, bbox_inches='tight')
    print("✅ Visualization saved as 'ml_analysis_results.png'")

    # Test HTTP request capability
    print("\n🌐 Testing HTTP Requests...")
    try:
        response = requests.get('https://httpbin.org/json', timeout=5)
        if response.status_code == 200:
            data = response.json()
            print(f"✅ HTTP request successful: {data.get('slideshow', {}).get('title', 'Test')}")
        else:
            print(f"⚠️  HTTP request returned status: {response.status_code}")
    except Exception as e:
        print(f"⚠️  HTTP request failed: {e}")

    print("\n🎉 ML Analysis Complete!")
    print("✨ All ML packages working from local lib/ directory")
    print(f"📁 Dependencies packaged in: {os.path.join(os.getcwd(), 'lib')}")

    return 0

if __name__ == "__main__":
    exit(main())