#!/usr/bin/env groovy
pipeline {
    agent any

    options {
        buildDiscarder(logRotator(numToKeepStr:'20'))
        timestamps()
    }

    parameters {
        string(name: 'UPLOAD_IMAGE', defaultValue: "False", description: "Should this build upload the Docker image?")
    }

    stages {
        stage("Clean workspace") {
            deleteDir()
        }
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
            when {
                expression { params.UPLOAD_IMAGE == "true" }
            }
            steps {
                sh 'make container'
            }
        }
    }
}
