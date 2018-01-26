#!/usr/bin/env groovy
pipeline {
    agent any

    parameters {
        string(name: 'UPLOAD_IMAGE', defaultValue: "False", description: "Should this build upload the Docker image?"
    }

    stages {
        stage("Test") {
            steps {
                sh 'make test'
            }
        }
        stage("Build executables") {
            steps {
                sh 'make all'
            }
        }
        stage("Build container") {
            steps {
                sh 'make container'
            }
        }
    }
}
