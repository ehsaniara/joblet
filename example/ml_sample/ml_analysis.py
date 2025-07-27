#!/usr/bin/env python3
import pandas as pd
import numpy as np
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestRegressor
from sklearn.metrics import mean_squared_error, r2_score
import matplotlib.pyplot as plt
import seaborn as sns
import os

print("🤖 Starting ML Analysis Pipeline")
print("=" * 50)

# Generate synthetic dataset
np.random.seed(42)
n_samples = 1000

data = {
    'feature1': np.random.normal(100, 15, n_samples),
    'feature2': np.random.normal(50, 10, n_samples),
    'feature3': np.random.uniform(0, 100, n_samples),
    'target': np.zeros(n_samples)
}

# Create target with some relationship to features
data['target'] = (
        2 * data['feature1'] +
        3 * data['feature2'] -
        0.5 * data['feature3'] +
        np.random.normal(0, 10, n_samples)
)

df = pd.DataFrame(data)

print(f"\n📊 Dataset shape: {df.shape}")
print("\nDataset summary:")
print(df.describe())

# Split data
X = df[['feature1', 'feature2', 'feature3']]
y = df['target']
X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

# Train model
print("\n🎯 Training Random Forest model...")
model = RandomForestRegressor(n_estimators=100, random_state=42)
model.fit(X_train, y_train)

# Predictions
y_pred = model.predict(X_test)

# Evaluation
mse = mean_squared_error(y_test, y_pred)
r2 = r2_score(y_test, y_pred)

print(f"\n📈 Model Performance:")
print(f"Mean Squared Error: {mse:.2f}")
print(f"R² Score: {r2:.3f}")

# Feature importance
feature_importance = pd.DataFrame({
    'feature': X.columns,
    'importance': model.feature_importances_
}).sort_values('importance', ascending=False)

print("\n🔍 Feature Importance:")
print(feature_importance)

# Create visualizations
plt.figure(figsize=(12, 8))

# Subplot 1: Feature Importance
plt.subplot(2, 2, 1)
sns.barplot(data=feature_importance, x='importance', y='feature')
plt.title('Feature Importance')

# Subplot 2: Actual vs Predicted
plt.subplot(2, 2, 2)
plt.scatter(y_test, y_pred, alpha=0.5)
plt.plot([y_test.min(), y_test.max()], [y_test.min(), y_test.max()], 'r--', lw=2)
plt.xlabel('Actual')
plt.ylabel('Predicted')
plt.title('Actual vs Predicted Values')

# Subplot 3: Residuals
plt.subplot(2, 2, 3)
residuals = y_test - y_pred
plt.scatter(y_pred, residuals, alpha=0.5)
plt.axhline(y=0, color='r', linestyle='--')
plt.xlabel('Predicted')
plt.ylabel('Residuals')
plt.title('Residual Plot')

# Subplot 4: Distribution of Residuals
plt.subplot(2, 2, 4)
sns.histplot(residuals, kde=True)
plt.xlabel('Residuals')
plt.title('Distribution of Residuals')

plt.tight_layout()
plt.savefig('ml_analysis_results.png', dpi=300, bbox_inches='tight')
print("\n📊 Visualizations saved to ml_analysis_results.png")

print("\n✅ ML Analysis Pipeline completed successfully!")