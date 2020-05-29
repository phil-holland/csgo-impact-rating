<p align="center">
  <img src="https://i.imgur.com/78yK1sr.png" />
  <br>
  <i>A probabilistic player rating system for Counter Strike: Global Offensive, powered by machine learning</i>
</p>

---

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

<p align="center">
  <img src="https://i.imgur.com/EBbyDLv.png" />
  <a href='#how-it-works'>How it Works</a> • <a href='#prediction-model'>Prediction Model</a> • <a href='#download'>Download</a> • <a href='#download'>Built With</a>
</p>

## How it Works

Impact Rating uses a machine learning model trained on a large amount of historical data to **predict the probable winner** of a given CS:GO round, based on the current state of the round. A **player's rating** is then calculated as the amount by which their actions shift the likelihood of their team winning the round. Therefore, **players are rewarded purely for making plays that improve their team's chance of winning the current round**. Conversely, negative rating is given when a player's actions reduce their team's chance of winning the round.

Two simplified examples of in-game scenarios are shown in the diagram below. Both describe a single CT player getting a triple kill, but they only receive impact rating in one scenario. In the first, the CTs are left in a 5v2 situation in which they are highly favoured to subsequently win the round. In the second, the remaining CTs are forfeiting the round, not attempting a late retake. A triple kill at this point **does not alter the CT team's chance of winning the round**.

![](https://i.imgur.com/vEMUxnD.png)

Internally, the state of a round at any given time is captured by the following features:

- CT players alive `[0, 5]`
- T players alive `[0, 5]`
- Mean health of CT players `[0.0, 100.0]`
- Mean health of T players `[0.0, 100.0]`
- Mean value of CT equipment `[0.0, ∞]`
- Mean value of T equipment `[0.0, ∞]`
- Whether the bomb has been planted `[true, false]`
- Whether the bomb has been defused `[true, false]` - *this is required to reward players for winning a round by defusing*
- Round time elapsed (in seconds) `[0.0, 115.0]` - *following a bomb plant, this field represents the time elapsed since the plant, instead of since the round start*

Inputs for each of these features are passed to the machine learning model, which returns a single floating-point value between `0.0` and `1.0` as the round winner prediction. The value can be directly interpreted as the probability that the round will be won by the T side. 

For example, a returned value of `0.34` represents a predicted **34% chance** of a **T side** round win, and a **66% chance** of a **CT side** round win.

## Prediction Model

*TODO*

## Download

*TODO*

## Built With

- [demoinfocs-golang](https://github.com/markus-wa/demoinfocs-golang) - used to parse CS:GO demo files
- [cobra](github.com/spf13/cobra) - used to build the command line interface
- [pb (v3)](github.com/cheggaaa/pb/v3) - used for progress visualisation
- [LightGBM](https://lightgbm.readthedocs.io/en/latest/) - used for model training/round outcome prediction