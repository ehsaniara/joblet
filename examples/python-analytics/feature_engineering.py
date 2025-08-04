#!/usr/bin/env python3
import pandas as pd
import numpy as np
import os
from sklearn.preprocessing import MinMaxScaler
import time

def process_chunk():
    chunk_id = int(os.getenv('CHUNK_ID', '1'))
    total_chunks = int(os.getenv('TOTAL_CHUNKS', '1'))
    
    print(f"Processing chunk {chunk_id} of {total_chunks}")
    
    # Load data chunk
    data_dir = '/volumes/analytics-data/raw'
    df = pd.read_csv(f'{data_dir}/data_chunk_{chunk_id}.csv')
    
    # Feature engineering operations
    print("Creating time-based features...")
    df['hour'] = pd.to_datetime(df['timestamp']).dt.hour
    df['day_of_week'] = pd.to_datetime(df['timestamp']).dt.dayofweek
    df['is_weekend'] = df['day_of_week'].isin([5, 6])
    
    print("Creating aggregate features...")
    df['rolling_mean_7d'] = df.groupby('user_id')['value'].transform(
        lambda x: x.rolling(window=7, min_periods=1).mean()
    )
    
    df['rolling_std_7d'] = df.groupby('user_id')['value'].transform(
        lambda x: x.rolling(window=7, min_periods=1).std()
    )
    
    # Scaling numerical features
    numerical_cols = ['value', 'rolling_mean_7d', 'rolling_std_7d']
    scaler = MinMaxScaler()
    df[numerical_cols] = scaler.fit_transform(df[numerical_cols])
    
    # Save processed chunk
    output_dir = '/volumes/analytics-data/processed'
    os.makedirs(output_dir, exist_ok=True)
    df.to_csv(f'{output_dir}/features_chunk_{chunk_id}.csv', index=False)
    
    print(f"Chunk {chunk_id} completed: {len(df)} records processed")

if __name__ == "__main__":
    process_chunk()