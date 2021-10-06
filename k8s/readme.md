# CQL PROXY with Kubernetes for Astra DB

Few things to keep a note of when creating the deployment file in kubernetes for running cql-proxy with Astra DB. 

### 1. **Create cql-proxy.yaml**

- **Args**:  adding the args in the right format. Make sure to add the Client ID and Client Secret for your Astra Database.   

      command: ["./cql-proxy"]
        args: ["--bundle=/tmp/scb.zip","--username=Client ID","--password=Client Secret"]

- Follow this [documentation](https://docs.datastax.com/en/astra/docs/manage-application-tokens.html#_create_application_token) to generate a token with Client ID and Client Secret. 

- **volumneMounts**: add `/tmp/` as mount

       volumeMounts:
        - name: my-cm-vol
          mountPath: /tmp/

- **volumne**: add `config` which is the configmap file. 

       volumes:
        - name: my-cm-vol
          configMap:
            name: cql-proxy-configmap        
    
    **Note** : Keep the volumne name and volumeMount name same.

### 2. **Create a configmap**

Use the secure bundle zip name as `scb.zip`. Get the secure bundle from the connect page in Astra Database. 
      

      kubectl create configmap cql-proxy-configmap --from-file scb.zip 

    Check the configmap that was created. 

    $ kubectl describe configmap config
      
      Name:         config
      Namespace:    default
      Labels:       <none>
      Annotations:  <none>

      Data
      ====

      BinaryData
      ====
      scb.zip: 12311 bytes


### 3. **Create a k8s deployment and check logs**


      $ kubectl logs cql-proxy-deployment-1-76fb6dbf9d-9kvc9
