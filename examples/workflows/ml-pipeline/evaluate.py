#!/usr/bin/env python3
"""
ML Model evaluation script
"""
import json
import sys
import os
import math

def main():
    print("Starting model evaluation...")
    
    # Read trained model
    if not os.path.exists("ml_data/trained_model.json"):
        print("Error: trained_model.json not found!")
        sys.exit(1)
    
    # Read test data (use selected features as test set)
    if not os.path.exists("ml_data/selected_features.json"):
        print("Error: selected_features.json not found!")
        sys.exit(1)
    
    with open("ml_data/trained_model.json", "r") as f:
        model = json.load(f)
    
    with open("ml_data/selected_features.json", "r") as f:
        test_data = json.load(f)
    
    # Evaluate model on test data
    features = test_data["features"]
    correct_predictions = 0
    predictions = []
    
    pos_centroid = model["parameters"]["positive_centroid"]
    neg_centroid = model["parameters"]["negative_centroid"]
    
    for sample in features:
        # Calculate distance to each centroid
        pos_dist = math.sqrt(
            (sample["feature1"] - pos_centroid["feature1"])**2 + 
            (sample["feature3"] - pos_centroid["feature3"])**2
        )
        neg_dist = math.sqrt(
            (sample["feature1"] - neg_centroid["feature1"])**2 + 
            (sample["feature3"] - neg_centroid["feature3"])**2
        )
        
        # Predict based on closest centroid
        predicted_label = 1 if pos_dist < neg_dist else 0
        actual_label = sample["label"]
        
        predictions.append({
            "id": sample["id"],
            "predicted": predicted_label,
            "actual": actual_label,
            "correct": predicted_label == actual_label
        })
        
        if predicted_label == actual_label:
            correct_predictions += 1
    
    accuracy = correct_predictions / len(features)
    
    # Create evaluation results
    evaluation = {
        "model_performance": {
            "accuracy": accuracy,
            "total_samples": len(features),
            "correct_predictions": correct_predictions,
            "incorrect_predictions": len(features) - correct_predictions
        },
        "predictions": predictions,
        "evaluation_timestamp": "2025-08-11T18:03:00Z",
        "model_ready_for_deployment": accuracy > 0.8
    }
    
    with open("ml_data/evaluation_results.json", "w") as f:
        json.dump(evaluation, f, indent=2)
    
    print(f"Evaluated model on {len(features)} samples")
    print(f"Accuracy: {accuracy:.2%}")
    print(f"Model ready for deployment: {evaluation['model_ready_for_deployment']}")
    print("Model evaluation completed successfully!")

if __name__ == "__main__":
    main()