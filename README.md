# Slowpoke-Oasis
All-In-One Tracker for Tasks, Groceries, and Budgets
> An all-in-one, modular daily companion: household ledger, smart grocery planning, and task tracker in a single app.

---

## Vision & Motivation

I want to create a "household" app, meaning it's a transparent, flexible, and fully user-centric companion for everyday life.

### Core Principles

* **100% Open-Source:** Full transparency and an open development process.
* **No Premium Version:** All features are freely available to all users.
* **Data Economy & Privacy First:** No background data collection or tracking.
* **No AI Gimmicks:** There won't be modules that use AI-features (i.e. AI creates recipes from leftover groceries)
* **Self-Hosting:** Future features that need a server are self-hosted (e.g., via Docker).
* **Gamification:** Integrated RPG aspects motivate you when completing daily chores, sticking to your budget.

---

## Planned Modules

### 1. Household Ledger (Finance)
* **Transactions:** Easily create and categorize income and expenses.
* **Flexible Budgets:** Budgets can be adjusted to actual salary receipt dates (e.g., 24th to 24th of the following month) instead of rigidly locked to the beginning of the month.
* **Categories & Analytics:** Clear overview of all expenses.
* **Banking Import/Export:** Future import of transactions from banking apps planned.

### 2. Grocery List & Recipe Planner
* **Grocery & Price Tracker:** Track groceries including package sizes and prices.
* **Intelligent Recipe Integration:** When adding recipes to the grocery list, the app automatically calculates the required package sizes.
* **Quantity Optimization:** If two recipes together require an ingredient (e.g., 150g + 50g tomato paste), the app recognizes the total demand and prevents unnecessary duplicate purchases.

### 3. Task Tracker & Reward System
* **Task Management:** Easily record, categorize, and prioritize daily tasks.
* **Difficulty Levels:** Individual grading of tasks.
* **Gamification & Rewards:** Higher difficulties yield more reward points. Collected points can be redeemed for personal "guilty pleasures" via an internal reward system.

---

## Planned Tech Stack

* **Frontend:** React Native (for Android & Cross-Platform)
* **Backend:** Go (RESTful API for lean performance)
* **Data Storage:** Hybrid approach with SQLite (relational for finances) and local JSON documents (for flexible recipes & lists)
* **Deployment:** Docker & Docker Compose for easy self-hosting

---

## Contributing

Since this project grows step-by-step, contributors, ideas, and pull requests are very welcome! 
---

## 📄 License

This project is licensed under the **AGPL-3.0 License**.
