#!/usr/bin/env python3
import pandas as pd
import numpy as np
from sklearn.cluster import KMeans
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import silhouette_score
import joblib
import os
from datetime import datetime

def train_customer_segmentation():
    print("Starting Customer Segmentation Training")
    
    # Load customer data
    df = pd.read_csv('/volumes/analytics-data/customers.csv')
    
    # Feature engineering
    features = ['age', 'income', 'spending_score', 'purchase_frequency']
    X = df[features].copy()
    
    # Handle missing values
    X = X.fillna(X.mean())
    
    # Scale features
    scaler = StandardScaler()
    X_scaled = scaler.fit_transform(X)
    
    # Determine optimal clusters using elbow method
    inertias = []
    silhouette_scores = []
    k_range = range(2, 11)
    
    print("Finding optimal number of clusters...")
    for k in k_range:
        kmeans = KMeans(n_clusters=k, random_state=42, n_init=10)
        cluster_labels = kmeans.fit_predict(X_scaled)
        
        inertias.append(kmeans.inertia_)
        sil_score = silhouette_score(X_scaled, cluster_labels)
        silhouette_scores.append(sil_score)
        print(f"k={k}: Silhouette Score = {sil_score:.3f}")
    
    # Select best k (highest silhouette score)
    best_k = k_range[np.argmax(silhouette_scores)]
    print(f"\nOptimal clusters: {best_k}")
    
    # Train final model
    final_model = KMeans(n_clusters=best_k, random_state=42, n_init=10)
    cluster_labels = final_model.fit_predict(X_scaled)
    
    # Add cluster labels to dataframe
    df['cluster'] = cluster_labels
    
    # Analyze clusters
    print("\nCluster Analysis:")
    cluster_summary = df.groupby('cluster')[features].mean()
    print(cluster_summary)
    
    # Save model and results
    model_dir = '/volumes/ml-models'
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    model_version = os.getenv('MODEL_VERSION', 'v1.0')
    
    # Save model artifacts
    joblib.dump(final_model, f'{model_dir}/clustering_model_{model_version}_{timestamp}.pkl')
    joblib.dump(scaler, f'{model_dir}/scaler_{model_version}_{timestamp}.pkl')
    
    # Save results
    df.to_csv(f'{model_dir}/segmented_customers_{timestamp}.csv', index=False)
    cluster_summary.to_csv(f'{model_dir}/cluster_summary_{timestamp}.csv')
    
    # Save metadata
    metadata = {
        'model_version': model_version,
        'timestamp': timestamp,
        'n_clusters': best_k,
        'silhouette_score': max(silhouette_scores),
        'features': features,
        'n_samples': len(df)
    }
    
    import json
    with open(f'{model_dir}/model_metadata_{timestamp}.json', 'w') as f:
        json.dump(metadata, f, indent=2)
    
    print(f"\nModel saved: clustering_model_{model_version}_{timestamp}.pkl")
    print(f"Silhouette Score: {max(silhouette_scores):.3f}")
    
    return final_model, scaler, metadata

if __name__ == "__main__":
    train_customer_segmentation()