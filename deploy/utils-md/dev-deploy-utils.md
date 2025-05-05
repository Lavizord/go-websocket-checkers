## Docker

    docker attach <container_id>

Or use an interactive shell:

    docker exec -it <container_id> /bin/sh  
    docker logs --since 5m <container_id>


```bash
// stop containers
docker stop $(docker ps -aq) 
// removes containers
docker rm $(docker ps -aq) 
// removes images
docker rmi $(docker images -aq) 
```

Or for windows:

    FOR /F "tokens=*" %i IN ('docker ps -aq') DO docker stop %i
    FOR /F "tokens=*" %i IN ('docker ps -qa') DO docker rm -v -f %i
    FOR /F "tokens=*" %i IN ('docker images -aq') DO docker rmi -f %i 


To restart docker containers in the server 

    docker restart $(docker ps -q)


docker exec -it redis redis-cli



### Clean Up space
#### Docker (This is the big one!)

Stop and remove all containers, networks, volumes

    docker-compose down --rmi all --volumes --remove-orphans

Remove all unused containers, networks, images (both dangling and unreferenced)
    
    docker system prune -a --volumes --force

Remove all Docker data (nuclear option - be careful!)

    docker system prune --all --force --volumes


#### Git

    git fetch --prune

You can clean up your local Git repository by deleting all branches except the current one and main with these commands:

    git branch | grep -vE "(main|\*)" | xargs git branch -D


#### GOAT - For building the server:

docker-compose -f docker-compose.dev.yml up --build -d



## Transfer Files to Server:

scp -i pvp.pem -r C:\Projetos\SSH-AWS\damas-server\SSL\* ec2-user@remote_host:/home/ec2-user/ssl/

## Reds 

Exex the redis cli container

    docker exec -it redis redis-cli


## PostgreSQL

### Clean up DB Volume
When the containers start up, if there is an existing database, the init script will be lost.
These are the steps to remove the docker volume to create the database from scratch.

1. Stop the containers:

```
docker-compose down
```

For staging:

```
docker-compose -f docker-compose.dev.nginx-local.yml down
```

2. Remove the volume:

```
docker volume rm go-websocket-checkers_postgres_data
```

For staging:
```
docker volume rm -f checkers-server_postgres_data
```

3. And then we start up the containers again:

```
docker-compose up -d
```

For staging:
```
docker-compose -f docker-compose.dev.nginx-local.yml up -d
```


### Enter postgres cmd

docker exec -it postgres psql -U sa -d checkers

### Check table names

SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';