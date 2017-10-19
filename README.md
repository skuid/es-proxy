# es-proxy

es-proxy is a proxy to AWS's ElasticSearch that signs request with a given set of AWS credentials.

## Usage

```
export AWS_ACCESS_KEY_ID=<key-id>
export AWS_SECRET_ACCESS_KEY=<access-key>
export AWS_REGION=<region>
./aws-signing-proxy -domain search-my-cluster.us-west-2.es.amazonaws.com
```

## Help

```
Usage of ./es-proxy:
      --domain string   The elasticsearch domain to proxy
      --port int        Listening port for proxy (default 8080)
      --region string   AWS region for credentials (default "us-west-2")
```

## Contributing

All code in the `/vendor` director is managed by [`dep`](https://github.com/golang/dep)

## Wishlist

- [ ] Add tests/CI
- [ ] Add docker builds to CI

## License
MIT License. See [License](/LICENSE) for full text
