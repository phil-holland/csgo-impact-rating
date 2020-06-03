import click
import os
import glob
import json
import sklearn
import optuna
import lightgbm as lgb
import numpy as np
from shutil import copyfile

train_data = None
val_data = None
dtrain = None
dvalid = None

@click.command()
@click.option('--train', '-t', type=click.Path(resolve_path=True, file_okay=True, dir_okay=False, exists=True), required=True)
@click.option('--val', '-v', type=click.Path(resolve_path=True, file_okay=True, dir_okay=False, exists=True), required=True)
@click.option('--pruning-warmup-rounds', '-w', type=int, default=50, show_default=True)
@click.option('--num-trials', '-n', type=int, default=100, show_default=True)
def train_lightgbm(train, val, pruning_warmup_rounds, num_trials):
    global train_data, val_data, dtrain, dvalid

    print('Loading data from csv files')
    train_data = np.genfromtxt(train, delimiter=',', skip_header=1)
    val_data = np.genfromtxt(val, delimiter=',', skip_header=1)

    feature_names = ['aliveCt','aliveT','bombPlanted','bombDefused','meanHealthCt','meanHealthT','meanValueCT','meanValueT','roundTime']

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

    if os.path.exists('./models'):
        print('Removing past model files')
        files = glob.glob('./models/*')
        for f in files:
            os.remove(f)
    else:
        print('Creating ./models directory')
        os.makedirs('./models')

    print('Starting Optuna study')
    study = optuna.create_study(
        pruner=optuna.pruners.MedianPruner(n_warmup_steps=pruning_warmup_rounds), direction='maximize'
    )
    study.optimize(lambda trial: objective(trial), n_trials=num_trials)

    print('Number of finished trials: {}'.format(len(study.trials)))
    print('Best trial:')
    trial = study.best_trial
    print('  AUC: {}'.format(trial.value))
    print('  Params: ')
    for key, value in trial.params.items():
        print('    {}: {}'.format(key, value))

    print('Copying best performing LightGBM model file to ./LightGBM_model.txt')
    copyfile('./models/LightGBM_model_%03d.txt' % study.best_trial.number, './LightGBM_model.txt')

def objective(trial):
    global train_data, val_data, dtrain, dvalid

    param = {
        'objective': 'binary',
        'metric': 'auc',
        'verbosity': -1,
        'boosting_type': 'gbdt',
        'learning_rate': 0.1,
        'num_leaves': 50,
        'lambda_l1': trial.suggest_loguniform('lambda_l1', 1e-8, 10.0),
        'lambda_l2': trial.suggest_loguniform('lambda_l2', 1e-8, 10.0),
        'feature_fraction': trial.suggest_uniform('feature_fraction', 0.4, 1.0),
        'bagging_fraction': trial.suggest_uniform('bagging_fraction', 0.4, 1.0),
        'bagging_freq': trial.suggest_int('bagging_freq', 1, 7),
        'min_child_samples': trial.suggest_int('min_child_samples', 5, 100),
    }

    # Add a callback for pruning
    pruning_callback = optuna.integration.LightGBMPruningCallback(trial, 'auc', 'eval')

    # start training
    gbm = lgb.train(
        param,
        dtrain,
        num_boost_round=250,
        early_stopping_rounds=30,
        valid_sets=[dvalid, dtrain],
        valid_names=['eval', 'train'],
        verbose_eval=False,
        categorical_feature=['bombPlanted','bombDefused'],
        callbacks=[pruning_callback]
    )

    out_path = './models/LightGBM_model_%03d.txt' % trial.number
    print('Writing LightGBM model out to', out_path)
    print('Training AUC:\t', gbm.best_score['train']['auc'])
    print('Evaluation AUC:\t', gbm.best_score['eval']['auc'])

    with open(out_path, 'w') as f:
        f.write(gbm.model_to_string())

    return gbm.best_score['eval']['auc']

if __name__ == '__main__':
    train_lightgbm() # pylint: disable=no-value-for-parameter