# Proyect ClaimClam Podcast Assessment

This repository contains:

-   Backend in **Golang**
-   Frontend in **Next.js** (submodule `web`)
-   Execution using **Docker Compose**

This README includes both manual steps to quickly set up and run everything.

---

# Clone the main repository

-   git clone git@github.com:MarmaduX/claimclam-podcast-assessment.git
-   cd claimclam-podcast-assessment

# Initialize and update the web submodule

git submodule update --init --recursive

# Launch everything with Docker Compose

docker-compose up --build
