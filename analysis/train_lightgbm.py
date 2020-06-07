import click
import os
import glob
import json
import sklearn
import optuna
import lightgbm as lgb
import numpy as np
from shutil import copyfile
from datetime import datetime

dtrain = None
dvalid = None

@click.command()
@click.option('--train', '-t', type=click.Path(resolve_path=True, file_okay=True, dir_okay=False, exists=True), required=True)
@click.option('--val', '-v', type=click.Path(resolve_path=True, file_okay=True, dir_okay=False, exists=True), required=True)
@click.option('--num-trials', '-n', type=int, default=100, show_default=True)
def train_lightgbm(train, val, num_trials):
    global dtrain, dvalid

    print('Loading data from csv files')
    train_data = np.genfromtxt(train, delimiter=',', skip_header=1)
    val_data = np.genfromtxt(val, delimiter=',', skip_header=1)

    feature_names = ['aliveCt','aliveT','meanHealthCt','meanHealthT','meanValueCT','meanValueT','roundTime','bombTime','bombDefused']

    dtrain = lgb.Dataset(
        data=train_data[:,1:],
        label=train_data[:,:1].flatten(),
        feature_name=feature_names,
        categorical_feature=None
    )
    
    dvalid = lgb.Dataset(
        data=val_data[:,1:],
        label=val_data[:,:1].flatten(),
        feature_name=feature_names,
        categorical_feature=None
    )

    # clear or create output directories
    if os.path.exists('./models'):
        print('Removing old model files from ./models directory')
        files = glob.glob('./models/*')
        for f in files:
            os.remove(f)
    else:
        print('Creating ./models directory')
        os.makedirs('./models')

    if os.path.exists('./trials'):
        print('Removing old trial files from ./trials directory')
        files = glob.glob('./trials/*')
        for f in files:
            os.remove(f)
    else:
        print('Creating ./trials directory')
        os.makedirs('./trials')

    if not os.path.exists('./studies'):
        print('Creating ./studies directory')
        os.makedirs('./studies')

    print('Starting Optuna study')
    study = optuna.create_study(
        pruner=optuna.pruners.MedianPruner(n_warmup_steps=20),
        direction='minimize'
    )
    study.optimize(lambda trial: objective(trial), n_trials=num_trials, )

    print('Number of finished trials: {}'.format(len(study.trials)))
    trial = study.best_trial
    print('Best trial: #{}'.format(trial.number))
    print('  Log-loss: {}'.format(trial.value))
    print('  Params: ')
    for key, value in trial.params.items():
        print('    {}: {}'.format(key, value))

    print('Writing study results to ./optuna_study.csv')
    df = study.trials_dataframe()
    df.to_csv('./optuna_study.csv')

    out_study_file = './studies/optuna_study_{:%Y-%m-%d_%H-%M-%S}.csv'.format(datetime.now())
    print('Copying study results to', out_study_file)
    copyfile('./optuna_study.csv', out_study_file)

    print('Copying best performing LightGBM model file to ./LightGBM_model.txt')
    copyfile('./models/LightGBM_model_%03d.txt' % study.best_trial.number, './LightGBM_model.txt')

def objective(trial):
    global dtrain, dvalid

    param = {
        'objective': 'binary',
        'metric': 'binary_logloss,auc',
        'verbosity': -1,
        'boosting_type': 'gbdt',
        'learning_rate': 0.01,
        'num_leaves': trial.suggest_int('num_leaves', 7, 1024),
        'max_depth': trial.suggest_int('max_depth', 2, 64),
        'lambda_l1': trial.suggest_loguniform('lambda_l1', 1e-8, 10.0),
        'lambda_l2': trial.suggest_loguniform('lambda_l2', 1e-8, 1.0),
        'feature_fraction': trial.suggest_uniform('feature_fraction', 0.4, 1.0),
        'bagging_fraction': trial.suggest_uniform('bagging_fraction', 0.4, 1.0),
        'bagging_freq': trial.suggest_int('bagging_freq', 1, 10),
        'min_child_samples': trial.suggest_int('min_child_samples', 5, 100),
    }

    # Add a callback for pruning
    pruning_callback = optuna.integration.LightGBMPruningCallback(trial, 'binary_logloss', 'val')

    # save score results to a dictionary
    results = {}

    # start training
    gbm = lgb.train(
        param,
        dtrain,
        num_boost_round=100000,
        early_stopping_rounds=50,
        valid_sets=[dvalid, dtrain],
        valid_names=['val', 'train'],
        verbose_eval=True,
        categorical_feature=['bombDefused'],
        callbacks=[pruning_callback, lgb.record_evaluation(results)]
    )

    print('Training log loss:\t', gbm.best_score['train']['binary_logloss'])
    print('Validation log loss:\t', gbm.best_score['val']['binary_logloss'])

    out_path = './models/LightGBM_model_%03d.txt' % trial.number
    print('Writing LightGBM model out to', out_path)

    with open(out_path, 'w', newline='\n') as f:
        f.write(gbm.model_to_string())

    out_path = './trials/trial_%03d.csv' % trial.number
    print('Writing trial results out to', out_path)
    with open(out_path, 'w') as f:
        f.write('round,train_logloss,val_logloss,train_auc,val_auc\n')
        for r in range(len(results['train']['binary_logloss'])):
            f.write(
                str(r) + ',' + 
                str(results['train']['binary_logloss'][r]) + ',' + 
                str(results['val']['binary_logloss'][r]) + ',' + 
                str(results['train']['auc'][r]) + ',' +
                str(results['val']['auc'][r]) + '\n'
            )

    return gbm.best_score['val']['binary_logloss']

if __name__ == '__main__':
    train_lightgbm() # pylint: disable=no-value-for-parameter