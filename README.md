Cigarette Shop 

Project Description

This project is a simple web application and API for managing a cigarette shop. Users can browse an assortment of cigarettes, add items to a cart, view the cart, remove items, and clear the cart entirely. The application is backed by MongoDB for storing product assortment and cart data.

Team Members

Damir,Ilyas,Adilbek

![image](https://github.com/user-attachments/assets/6d2a262d-2a5e-4763-bfac-3758ba26529d)




Setup and Run Instructions

Prerequisites

Go installed (version 1.23.4)

MongoDB installed and running locally

Git installed

Steps to Run the Project

Clone the Repository:

git clone https://github.com/master367/Cigarette_shop


Install Dependencies:
Use go mod to install necessary Go dependencies.

go mod tidy

Set Up MongoDB:

Ensure MongoDB is running on localhost:27017.

Create a database named Shop.

Create two collections: assortment and cart.

Populate the assortment collection with sample data if needed.

Run the Server:

go run main.go

Access the Application:
Open your browser and go to http://localhost:8080.

API Endpoints

Assortment

GET /cigarettes: Retrieve all available cigarettes.

Cart

GET /cart: View all items in the cart.

POST /cart/add: Add a cigarette to the cart. (JSON Body: { "brand": "Brand Name", "price": 10.0 })

POST /cart/remove: Remove a cigarette from the cart. (JSON Body: { "brand": "Brand Name" })

POST /cart/clear: Clear all items from the cart.

Frontend

GET /: Serve the HTML page (index.html) for the main interface.

Tools and Resources

Backend Framework: Gorilla Mux

Database: MongoDB

Template Engine: Go HTML Templates

Language: Go

Other Tools:

IntelliJ IDEA or any Go IDE

Postman (for API testing)

GitHub (for version control)
