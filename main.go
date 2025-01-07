package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/time/rate"
	"html/template"
	"net/http"
	"os"
	"sync"
	"time"
)

type Cigarette struct {
	Brand    string  `json:"brand,omitempty" bson:"brand,omitempty"`
	Price    float64 `json:"price,omitempty" bson:"price,omitempty"`
	Type     string  `json:"type,omitempty" bson:"type,omitempty"`
	Category string  `json:"category,omitempty" bson:"category,omitempty"`
}

type Cart struct {
	ID    string      `json:"id,omitempty" bson:"_id,omitempty"`
	Items []Cigarette `json:"items,omitempty" bson:"items,omitempty"`
	Total float64     `json:"total,omitempty" bson:"total,omitempty"`
}

var limiter = make(map[string]*rate.Limiter)
var limiterLock = &sync.Mutex{}
var log = logrus.New()
var client *mongo.Client
var assortmentCollection *mongo.Collection
var cartCollection *mongo.Collection

func rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем IP-адрес клиента
		ip := r.RemoteAddr

		// Блокируем доступ к limiter для изменения
		limiterLock.Lock()
		defer limiterLock.Unlock()

		// Проверяем, существует ли уже лимитер для этого IP
		if _, exists := limiter[ip]; !exists {
			// Если нет, создаем новый лимитер с лимитом 5 запросов в минуту
			limiter[ip] = rate.NewLimiter(rate.Every(time.Minute), 5)
		}

		// Проверяем, может ли этот IP сделать запрос
		if !limiter[ip].Allow() {
			log.Printf("Rate limit exceeded for IP: %s", ip)
			http.Error(w, "Превышен лимит запросов", http.StatusTooManyRequests)
			return
		}

		// Если лимит не превышен, передаем управление следующему обработчику
		next.ServeHTTP(w, r)
	})
}

func init() {
	file, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logrus.Fatal(err)
	}
	log.SetOutput(file)
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	log.SetLevel(logrus.InfoLevel)
}

func connectToMongo() {
	var err error
	client, err = mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("Не удалось подключиться к MongoDB")
	}
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("MongoDB не отвечает")
	}
	log.Info("Успешно подключено к MongoDB")
	assortmentCollection = client.Database("Shop").Collection("assortment")
	cartCollection = client.Database("Shop").Collection("cart")
}

func getCigarettesWithFilters(w http.ResponseWriter, r *http.Request) {
	brand := r.URL.Query().Get("brand")
	sortField := r.URL.Query().Get("sortField")
	sortOrder := r.URL.Query().Get("sortOrder") // "asc" или "desc"
	limitParam := r.URL.Query().Get("limit")
	pageParam := r.URL.Query().Get("page")

	limit := 10
	page := 1
	if limitParam != "" {
		fmt.Sscanf(limitParam, "%d", &limit)
	}
	if pageParam != "" {
		fmt.Sscanf(pageParam, "%d", &page)
	}

	filter := bson.M{}
	if brand != "" {
		filter["brand"] = bson.M{"$regex": brand, "$options": "i"} // Регистронезависимый поиск
	}

	sort := bson.D{}
	if sortField != "" {
		sortDirection := 1
		if sortOrder == "desc" {
			sortDirection = -1
		}
		sort = append(sort, bson.E{Key: sortField, Value: sortDirection})
	}

	skip := (page - 1) * limit
	options := options.Find().SetSort(sort).SetLimit(int64(limit)).SetSkip(int64(skip))
	cursor, err := assortmentCollection.Find(context.Background(), filter, options)
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
	log.WithFields(logrus.Fields{
		"method": r.Method,
		"url":    r.URL.Path,
		"ip":     r.RemoteAddr,
	}).Info("Request received")

	var cigarette Cigarette
	err := json.NewDecoder(r.Body).Decode(&cigarette)
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Error decoding cigarette")
		http.Error(w, "Неверный ввод", http.StatusBadRequest)
		return
	}

	// Добавление сигареты в корзину
	_, err = cartCollection.InsertOne(context.Background(), cigarette)
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Error adding to cart")
		http.Error(w, "Ошибка при добавлении в корзину", http.StatusInternalServerError)
		return
	}

	log.Info("Cigarette added to cart successfully")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Сигарета добавлена в корзину")
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

func getLink(port int) string {
	return "http://localhost:" + fmt.Sprintf("%d", port)
}

func main() {

	connectToMongo()

	r := mux.NewRouter()
	r.HandleFunc("/add-to-cart", addCigaretteToCart)
	r.HandleFunc("/cigarettes", getCigarettesWithFilters).Methods("GET")
	r.HandleFunc("/cart", getCart).Methods("GET")
	r.Handle("/cart/add", rateLimit(http.HandlerFunc(addCigaretteToCart))).Methods("POST")
	r.HandleFunc("/cart/remove", removeItemFromCart).Methods("POST")
	r.HandleFunc("/cart/clear", clearCart).Methods("POST")
	r.HandleFunc("/cigarette", getCigaretteByBrand).Methods("GET")
	r.HandleFunc("/cigarette/update", updateCigarettePrice).Methods("POST")
	r.HandleFunc("/", serveHTML)

	log.Printf("Server started on %s", getLink(8080))
	log.Fatal(http.ListenAndServe(":8080", r))
}
