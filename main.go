package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"html/template"
	"log"
	"net/http"
)

type Cigarette struct {
	Brand string  `json:"brand,omitempty" bson:"brand,omitempty"`
	Price float64 `json:"price,omitempty" bson:"price,omitempty"`
}

type Cart struct {
	ID    string      `json:"id,omitempty" bson:"_id,omitempty"`
	Items []Cigarette `json:"items,omitempty" bson:"items,omitempty"`
	Total float64     `json:"total,omitempty" bson:"total,omitempty"`
}

var client *mongo.Client
var assortmentCollection *mongo.Collection
var cartCollection *mongo.Collection

func connectToMongo() {
	var err error
	client, err = mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal("MongoDB connection is not responding:", err)
	}
	fmt.Println("Connected to MongoDB")
	assortmentCollection = client.Database("Shop").Collection("assortment")
	cartCollection = client.Database("Shop").Collection("cart")
}

func getAllCigarettes(w http.ResponseWriter, r *http.Request) {
	cursor, err := assortmentCollection.Find(context.Background(), bson.D{})
	if err != nil {
		http.Error(w, "Error fetching cigarettes", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var cigarettes []Cigarette
	for cursor.Next(context.Background()) {
		var cigarette Cigarette
		err := cursor.Decode(&cigarette)
		if err != nil {
			http.Error(w, "Error decoding cigarette", http.StatusInternalServerError)
			return
		}
		cigarettes = append(cigarettes, cigarette)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cigarettes)
}

func addCigaretteToCart(w http.ResponseWriter, r *http.Request) {
	var cigarette Cigarette
	err := json.NewDecoder(r.Body).Decode(&cigarette)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err = cartCollection.InsertOne(context.Background(), cigarette)
	if err != nil {
		http.Error(w, "Error adding to cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Cigarette added to cart")
}

func getCart(w http.ResponseWriter, r *http.Request) {
	cursor, err := cartCollection.Find(context.Background(), bson.D{})
	if err != nil {
		http.Error(w, "Error fetching cart", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var cart []Cigarette
	for cursor.Next(context.Background()) {
		var item Cigarette
		err := cursor.Decode(&item)
		if err != nil {
			http.Error(w, "Error decoding cart item", http.StatusInternalServerError)
			return
		}
		cart = append(cart, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
}

func clearCart(w http.ResponseWriter, r *http.Request) {
	_, err := cartCollection.DeleteMany(context.Background(), bson.D{})
	if err != nil {
		http.Error(w, "Error clearing cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Cart cleared")
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, "Error loading HTML template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func removeItemFromCart(w http.ResponseWriter, r *http.Request) {
	var cigarette Cigarette
	err := json.NewDecoder(r.Body).Decode(&cigarette)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err = cartCollection.DeleteOne(context.Background(), bson.M{"brand": cigarette.Brand})
	if err != nil {
		http.Error(w, "Error deleting item from cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Cigarette removed from cart")
}

func main() {

	connectToMongo()

	r := mux.NewRouter()

	r.HandleFunc("/cigarettes", getAllCigarettes).Methods("GET")
	r.HandleFunc("/cart", getCart).Methods("GET")
	r.HandleFunc("/cart/add", addCigaretteToCart).Methods("POST")
	r.HandleFunc("/cart/remove", removeItemFromCart).Methods("POST")
	r.HandleFunc("/cart/clear", clearCart).Methods("POST")
	r.HandleFunc("/cigarette", getCigaretteByBrand).Methods("GET")
	r.HandleFunc("/cigarette/update", updateCigarettePrice).Methods("POST")
	r.HandleFunc("/", serveHTML)

	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func getCigaretteByBrand(w http.ResponseWriter, r *http.Request) {
	brand := r.URL.Query().Get("brand")
	var cigarette Cigarette
	err := assortmentCollection.FindOne(context.Background(), bson.M{"brand": brand}).Decode(&cigarette)
	if err != nil {
		http.Error(w, "Cigarette not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cigarette)
}

func updateCigarettePrice(w http.ResponseWriter, r *http.Request) {
	var updateData struct {
		Brand string  `json:"brand"`
		Price float64 `json:"price"`
	}
	err := json.NewDecoder(r.Body).Decode(&updateData)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err = assortmentCollection.UpdateOne(
		context.Background(),
		bson.M{"brand": updateData.Brand},
		bson.M{"$set": bson.M{"price": updateData.Price}},
	)
	if err != nil {
		http.Error(w, "Error updating price", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Price updated successfully")
}
