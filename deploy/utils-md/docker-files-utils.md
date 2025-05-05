# Testing dockerfile
## Broadcast Worker

To build the docker image, run from root:

``` 
    docker build -f cmd/broadcastworker/Dockerfile -t broadcastworker .
```


To run the image built:

``` 
    docker run --rm broadcastworker 

``` 


## Game Worker

To build the docker image, run from root:

``` 
    docker build -f cmd/gameworker/Dockerfile -t gameworker .
```


To run the image built:

``` 
    docker run --rm gameworker 

``` 


## Pstatus Worker

To build the docker image, run from root:

``` 
    docker build -f cmd/pstatusworker/Dockerfile -t pstatusworker .
```


To run the image built:

``` 
    docker run --rm pstatusworker 

``` 

## WebsocketApi Worker

To build the docker image, run from root:

``` 
    docker build -f cmd/wsapi/Dockerfile -t wsapi .
```


To run the image built:

``` 
    docker run --rm wsapi 

``` 


## RestApi Worker

To build the docker image, run from root:

``` 
    docker build -f cmd/restapiworker/Dockerfile -t restapiworker .
```


To run the image built:

``` 
    docker run --rm restapiworker 

``` 


## Room Worker

To build the docker image, run from root:

``` 
    docker build -f cmd/roomworker/Dockerfile -t roomworker .
```


To run the image built:

``` 
    docker run --rm roomworker 

``` 