# route53_register

Tiny Go CLI Program to register EC2 host in Route53

[![Build Status](https://travis-ci.org/reflog/route53_register.svg?branch=master)](https://travis-ci.org/reflog/route53_register)

# usage

```
Usage of ./route53_register:
  -cname
        wherether to create CNAME record instead of an A record. (will use public hostname instead of IP)
  -hostname string
        which name to use for the new entry
  -zonename string
        which zone to use for registering records
```

# use case

if you are using ECS and you have a service that's dynamically placed on some EC2 instance, you can add this command to your docker startup:

`route53_register -hostname my_service -zonename myzone.internal`
