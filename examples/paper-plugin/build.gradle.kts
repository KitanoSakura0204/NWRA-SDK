// Paper plugin skeleton for NoWork Account. Build: gradle build (Gradle 8.5+, JDK 21);
// the jar lands in build/libs. The SDK has no dependencies, so its sources are compiled in
// directly instead of being published and shaded.
plugins {
    java
}

group = "cyou.nowork.example"
version = "1.0.0"

repositories {
    mavenCentral()
    maven("https://repo.papermc.io/repository/maven-public/")
}

dependencies {
    compileOnly("io.papermc.paper:paper-api:1.21.1-R0.1-SNAPSHOT")
}

java {
    toolchain.languageVersion.set(JavaLanguageVersion.of(21))
}

sourceSets {
    main {
        java.srcDir("../../java/src/main/java")
    }
}

tasks.withType<JavaCompile>().configureEach {
    options.encoding = "UTF-8"
    options.release.set(21)
}

tasks.processResources {
    filesMatching("plugin.yml") {
        expand("version" to project.version)
    }
}
