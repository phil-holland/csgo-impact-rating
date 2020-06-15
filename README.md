<p align="center">
  <img src="https://i.imgur.com/78yK1sr.png" />
  <br>
  <i>A probabilistic player rating system for Counter Strike: Global Offensive, powered by machine learning</i>
</p>

---

[![Latest release](https://img.shields.io/github/v/release/Phil-Holland/csgo-impact-rating?label=release&sort=semver)](https://github.com/Phil-Holland/csgo-impact-rating/releases)
[![Build Status](https://travis-ci.org/Phil-Holland/csgo-impact-rating.svg?branch=master)](https://travis-ci.org/Phil-Holland/csgo-impact-rating)
[![Go Report Card](https://goreportcard.com/badge/github.com/Phil-Holland/csgo-impact-rating)](https://goreportcard.com/report/github.com/Phil-Holland/csgo-impact-rating)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

<p align="center">
  <img src="https://i.imgur.com/EBbyDLv.png" />
  <br>
  <a href='#how-it-works'>How it Works</a> • <a href='#prediction-model'>Prediction Model</a> • <a href='#download'>Download</a> • <a href='#usage'>Usage</a> • <a href='#built-with'>Built With</a>
</p>

## How it Works

Impact Rating uses a machine learning model trained on a large amount of historical data to **predict the probable winner** of a given CS:GO round, based on the current state of the round. A **player's Impact Rating** is then calculated as the amount by which their actions shift the predicted likelihood of their team winning the round. Therefore, **players are rewarded purely for making plays that improve their team's chance of winning the current round**. Conversely, negative Impact Rating is given when a player's actions reduce their team's chance of winning the round.

A player's performance over the course of a complete game can be summarised with their **Average Impact Rating (AIR)** - the mean of all single-round ratings.

### Example

Two simplified examples of in-game scenarios are shown in the diagram below. Both describe a single CT player getting a triple kill, but they only receive impact rating in one scenario. In the first, the CTs are left in a 5v2 situation in which they are highly favoured to subsequently win the round. In the second, the remaining CTs are forfeiting the round, not attempting a late retake. A triple kill at this point **does not alter the CT team's chance of winning the round**.

![](https://i.imgur.com/vEMUxnD.png)

### Case Study

An in-depth case study analysis of an individual match between Mousesports and Faze can be found here: [case study](https://nbviewer.jupyter.org/github/Phil-Holland/csgo-impact-rating/blob/master/model/02-case_study.ipynb). This analysis breaks down the events of a round, showing how model prediction values change accordingly, and the resulting effect on player Impact Ratings.

### Calculating Impact Rating

Internally, the state of a round at any given time is captured by the following features:

- The number of CT players alive
- The number of T players alive
- The mean health of CT players
- The mean health of T players
- The mean value of CT equipment
- The mean value of T equipment
- The round time elapsed (in seconds)
- The bomb time elapsed (in seconds) - *this is zero before the bomb is planted*
- Whether the bomb is currently being defused
- Whether the bomb has been defused  - *this is required to reward players for winning a round by defusing*

Inputs for each of these features are passed to the machine learning model, which returns a single non-integer value between `0.0` and `1.0` as the round winner prediction. This value can be directly interpreted as the **probability** that the round will be won by the **T-side**. 

> *For example, a returned value of `0.34` represents a predicted **34% chance** of a **T-side** round win, and a **66% chance** of a **CT-side** round win.*

This concept is applied to **every in-game event that causes a round's state to change** from the end of freezetime to the moment the round is won. Players that contributed to changing the round outcome prediction are rewarded with a **appropriate share of the percentage change** in their team's favour - the **sum of these values** over a particular round is their **Impact Rating for that round**.

The following action categories are considered when rewarding players with their share:

- **Dealing damage**
  - This rewards players who damage an opponent's health
  - This also punishes players for team-damage
- **Trade damage** *(if an opponent takes damage very soon after they themselves have damaged the player in question)*
  - This rewards players for taking damage so that a teammate can damage their attacker
- **Flash assist damage** *(if someone takes damage whilst blinded by a flashbang thrown by the player in question)*
  - This rewards players for flashing an enemy who then sustains damage
  - This also punishes players for team-flashing their teammate into taking damage
- **Successfully retaking**
  - This rewards players who win rounds by retaking and defusing the bomb - all living CTs are rewarded when the bomb is defused
  - This also punishes T-side players who cannot prevent a defuse whilst alive
- **Sustaining damage** *(being hurt)*
  - This punishes players who take damage themselves
- **Being alive at the end of a round** *(after time has run out or the bomb has exploded)*
  - This rewards players for forcing their opponent to save
  - This also punishes players for saving

## Prediction Model

Whilst the machine learning aspect of Impact Rating can in theory be implemented using any binary classification model, the code here has been written to target the [LightGBM framework](https://github.com/Microsoft/LightGBM). This is a framework used for gradient boosting decision trees (GBDT), and has been [shown to perform very well](https://github.com/microsoft/LightGBM/blob/master/docs/Experiments.rst) in binary classification problems. It has also been chosen for its lightweight nature, and ease of installation.

Model analysis and instructions for how to train a new model can be found here: [model analysis](model/README.md).

## Download

The latest Impact Rating distribution for your system can be downloaded from this Github project's release page (for 99% of Windows users, this means downloading the `csgo-impact-rating_win64.zip` file).

<p align="center">
  <a href="https://github.com/Phil-Holland/csgo-impact-rating/releases/latest" style="font-size: 1.25em">
    <b>Download Latest Version</b>
  </a>
</p>

Extract the executable and the `LightGBM_model.txt` file to a directory of your choosing - add this directory to the system path to access the executable from any location.

## Usage

CS:GO Impact Rating is distributed as a command line tool - it can be invoked only through a command line such as the Windows command prompt (cmd.exe) or a Linux shell.

```
Usage: csgo-impact-rating [OPTION]... [DEMO_FILE (.dem)]

Tags DEMO_FILE, creating a '.tagged.json' file in the same directory, which is
subsequently evaluated, producing an Impact Rating report which is written to
the console and a '.rating.json' file.

  -f, --force                Force the input demo file to be tagged, even if a
                             .tagged.json file already exists.
  -p, --pretty               Pretty-print the output .tagged.json file.
  -s, --eval-skip            Skip the evaluation process, only tag the input
                             demo file.
  -m, --eval-model string    The path to the LightGBM_model.txt file to use for
                             evaluation. If omitted, the application looks for
                             a file named "LightGBM_model.txt" in the same
                             directory as the executable.
  -v, --eval-verbosity int   Evaluation console verbosity level:
                              0 = do not print a report
                              1 = print only overall rating
                              2 = print overall & per-round ratings (default 2)
```

For general usage, the above command line flags can be ignored. For example, the following command will process and **produce player ratings** for a demo file named `example.dem` in the working directory:

```sh
csgo-impact-rating example.dem
```

A full per-player Impact Rating report will be shown in the console output.

### Processing Details

Processing consists of two internal stages: **tagging** and **evaluation**.

#### 1. Tagging:

First, the raw demo file is parsed from start to finish, creating a *"tagged file"* in the same directory as the input demo with the extension `.tagged.json`. This is a [JSON file](https://en.wikipedia.org/wiki/JSON) that describes the key events of the demo, each "tagged" with any players who have contributed to that event. A complete example of a single "tagged" event is shown below:

```json
{
  "tick": 354002,
  "type": "playerDamage",
  "scoreCT": 11,
  "scoreT": 8,
  "teamCT": { "id": 3, "name": "FaZe Clan" },
  "teamT": { "id": 2, "name": "mousesports" },
  "players": [
    { "steamID": 76561198083485506, "name": "woxic", "teamID": 2 },
    { "steamID": 76561198068422762, "name": "frozen","teamID": 2 },
    { "steamID": 76561197997351207, "name": "rain", "teamID": 3 },
    { "steamID": 76561198201620490, "name": "broky", "teamID": 3 },
    { "steamID": 76561197988627193, "name": "olofmeister", "teamID": 3 },
    { "steamID": 76561197991272318, "name": "ropz", "teamID": 2 },
    { "steamID": 76561197989430253, "name": "karrigan", "teamID": 2 },
    { "steamID": 76561197988539104, "name": "chrisJ", "teamID": 2 },
    { "steamID": 76561198039986599, "name": "coldzera", "teamID": 3 },
    { "steamID": 76561198041683378, "name": "NiKo", "teamID": 3 }
  ],
  "gameState": {
    "aliveCT": 4,
    "aliveT": 1,
    "meanHealthCT": 86.25,
    "meanHealthT": 100,
    "meanValueCT": 6312.5,
    "meanValueT": 5100,
    "roundTime": 39.375,
    "bombTime": 0,
    "bombDefused": false
  },
  "tags": [
    { "action": "damage", "player": 76561197991272318 },
    { "action": "flashAssist", "player": 76561197991272318 },
    { "action": "hurt", "player": 76561197988627193 }
  ],
  "roundWinner": 0
}
```

**Note:** if an tagged file already exists for the input demo, this stage is skipped by default.

#### 2. Evaluating: 

Secondly, each event saved in the tagged file is evaluated with the machine learning model, producing a predicted round outcome probability. These probabilities are then used to calculate player ratings for each round, and their overall average over all rounds. This is printed to the console window - an Average Impact Rating table for an example demo is shown below:

```
> Overall:

Team           Player         Average Impact (%)   |   Damage (%)    Flash Assists (%)    Trade Damage (%)    Retakes (%)    Damage Recv. (%)    Alive (%)
----           ------         ------------------   |   ----------    -----------------    ----------------    -----------    ----------------    ---------
FaZe Clan      olofmeister    -4.605               |   7.132         0.319                1.259               0.000          -13.293             -0.022
FaZe Clan      rain           -1.889               |   10.494        0.057                0.183               0.000          -12.608             -0.014
FaZe Clan      coldzera       4.640                |   19.461        -0.797               0.340               0.005          -14.307             -0.063
FaZe Clan      NiKo           1.437                |   8.293         0.081                2.151               0.005          -9.046              -0.047
FaZe Clan      broky          0.428                |   9.891         1.187                1.912               0.005          -12.569             0.001
mousesports    woxic          0.054                |   10.532        0.626                1.482               0.076          -12.715             0.053
mousesports    frozen         14.941               |   19.581        1.006                1.807               2.466          -9.887              -0.033
mousesports    ropz           -6.264               |   5.485         0.030                0.856               0.069          -12.761             0.057
mousesports    karrigan       -2.735               |   8.918         0.962                0.655               0.076          -13.370             0.024
mousesports    chrisJ         -3.252               |   9.492         0.000                1.592               0.145          -14.526             0.045
```

All calculated statistics are saved to a *"rating file"* with the extension `.rating.json` in the same directory as the input demo. Along with player rating summaries, this file contains the inferred probabilities at each event, and the changes in player ratings through each round.

## Built With

- [demoinfocs-golang](https://github.com/markus-wa/demoinfocs-golang) - used to parse CS:GO demo files
- [leaves](https://github.com/dmitryikh/leaves) - used to process LightGBM models internally
- [pflag](https://github.com/spf13/pflag) - used to build the command line interface
- [pb (v3)](https://github.com/cheggaaa/pb) - used for progress visualisation
- [LightGBM](https://github.com/Microsoft/LightGBM) - used for model training/round outcome prediction
- [Optuna](https://optuna.org/) - used for hyperparameter optimisation during training