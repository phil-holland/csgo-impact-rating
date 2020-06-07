# LightGBM Model Analysis

## Optimal Model

## Training a Custom Model

The Python scripts contained in this directory are for data preprocessing and LightGBM model training. To set up the Python environment it is recommended to create a new [Conda](https://docs.conda.io/en/latest/miniconda.html) virtual environment from the `environment.yml` file, like so:

```
conda env create -f environment.yml
```

This will create a new Python 3.7 virtual environment named `ir_analysis` with all required dependencies. This can be activated by running the following command:

```
conda activate ir_analysis
```

The following sections describe how to use the provided Python scripts to prepare a custom dataset to train your own LightGBM model.

### Checking for "Corrupt" Tag Files

CS:GO demo files sometimes contain malformed data, leading to errors occuring whilst parsing. To identify `.tagged.json` files which contain ticks with unexpected values for fields like `aliveCT`, `roundTime` etc., run the `find_corrupt.py` script. Simply invoke the script, passing the directory containing your `.tagged.json` files like so:

```
python find_corrupt.py /path/to/tagged/files/dir
```

The script will iterate through all `.tagged.json` files in the specified directory, and report any files which appear to have errors. These reported files can be removed from the directory, stopping them from being added to both the training and evaluation datasets.

### Creating Training/Evaluation CSVs

```
python create_train_val_csv.py /path/to/tagged/files/dir
```

### Training an Optimal LightGBM Model

```
python train_lightgbm.py -t train.csv -v val.csv -n 250
```

### Analysing Performance