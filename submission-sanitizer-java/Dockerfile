FROM maven:3.9.9-eclipse-temurin-17 AS build
WORKDIR /app

COPY pom.xml .
COPY src ./src

RUN mvn -q -DskipTests package

FROM eclipse-temurin:17-jre
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY --from=build /app/target/submission-sanitizer-0.0.1-SNAPSHOT.jar /app/submission-sanitizer.jar

EXPOSE 8090
ENTRYPOINT ["java", "-jar", "/app/submission-sanitizer.jar"]
