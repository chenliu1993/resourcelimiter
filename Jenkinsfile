pipeline {
    agent any
    stages {      
        stage('Start') {
            steps {
                echo 'Start
            }
        }
        stage('Build') {
            steps {
                   sh 'make build'
            }
        }
        stage('Test') {
            steps {
                echo 'make test'
            }
        }
    }
    post {
        always {
            script {
                echo 'Finished'
            }
        }
        
    }
}