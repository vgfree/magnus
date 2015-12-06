Magnus
======
Magnus leverages etcd to implement multi-leader high availability in AWS.

How Does It Work?
-----------------
Magnus runs across a set of machines that wish to participate in leadership
elections. State is stored in a single etcd directory. When a Magnus instance
comes online it checks etcd to see if there is an available leadership slot. If
so the Magnus instance will assign itself an available EIP (Elastic Network
Interface) from the configured pool and update the etcd state directory.

This is made possible by using etcd as a locking mechanism. Only one Magnus
instance is allowed to access the state directory at a time. Changes to the
state directory cause all Magnus instances to wake up and acquire a lock on the
directory. Leaders are thus elected on a first-come-first-serve basis.

When a leader is elected it will set a TTL on its key in etcd. When a leader
fails unexpectedly the TTL will expire and another Magnus instance may elect
itself.

Running Magnus
--------------
Mangus requires one or more etcd endpoints, a path to the etcd state key, and a
list of ENI ID's. It is thus run:

	magnus -endpoints=https://etcd.example.net/ -key=/magnus -interfaces=eni-124567

The etcd directory is created if it does not exist. In the above example it
would create the directory at  /magnus. When doing so a size key will be
created and set to 1. This is the number of leaders to allow. There should be
at least as many EIP ID's as there are leaders.
