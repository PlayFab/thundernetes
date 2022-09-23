Scalability Test
===

The tests here is for testing the scalability and performance of Thundernetes. It Uses openarena as testing game server to test scalability. It creates necessary functions to perform scaling up to 10 and 50 standing by gameservers respectively.

To run the whole test suite, simply by running `./scale-test.sh` on Linux.

You can also just run `source util.sh` and use `scale_up()` and `scale_clear()` function to run your customized tests.


```
./scale-test.sh

test 1: scale up to 10 game servers
gameserverbuild.mps.playfab.com/gameserverbuild-sample-openarena configured
gameserverbuild.mps.playfab.com/gameserverbuild-sample-openarena scaled

Scaled up: 10/10
Scale up time: 15s
gameserverbuild.mps.playfab.com/gameserverbuild-sample-openarena scaled
test 2: scale up to 50 game servers
gameserverbuild.mps.playfab.com/gameserverbuild-sample-openarena configured
gameserverbuild.mps.playfab.com/gameserverbuild-sample-openarena scaled

Scaled up: 50/50
Scale up time: 100s
gameserverbuild.mps.playfab.com/gameserverbuild-sample-openarena scaled
```