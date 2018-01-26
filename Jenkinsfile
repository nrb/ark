#!/usr/bin/env groovy
pipeline {
    agent any

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
