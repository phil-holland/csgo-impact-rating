<p align="center">
  <img src="https://i.imgur.com/78yK1sr.png" />
  <br>
  <i>A probabilistic rating system for Counter Strike: Global Offensive, powered by machine learning</i>
</p>

---

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

<p align="center">
  <img src="https://i.imgur.com/EBbyDLv.png" />
</p>

## How it Works

Impact Rating uses a machine learning model trained on a large amount of historical data to **predict the probable winner** of a given CS:GO round, based on the current state of the round. A player's rating is then calculated as the amount by which their actions shift the likelihood of their team winning the round. Therefore, **players are rewarded for making plays that improve their team's chance of winning the current round**.

Two simplified examples of in-game scenarios are shown in the diagram below. Both describe a single CT player getting a triple kill, but they only receive impact rating in one scenario. In the first, the CTs are left in a 5v2 situation in which they are highly favoured to subsequently win the round. In the second, the CTs are forfeiting the round, and not attempting a retake. A triple kill at this point **does not alter the CT team's chance of winning the round**.

![](https://i.imgur.com/QhzUsJB.png)

## Download

*TODO*

## Built With

- [demoinfocs-golang](https://github.com/markus-wa/demoinfocs-golang) - used to parse CS:GO demo files.
- [cobra](github.com/spf13/cobra) - used to build the command line interface.
- [pb (v3)](github.com/cheggaaa/pb/v3) - used for progress visualisation.
- [LightGBM](https://lightgbm.readthedocs.io/en/latest/) - used for model training/outcome prediction.